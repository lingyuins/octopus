package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebDAVClient WebDAV 客户端
type WebDAVClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

// maxWebDAVDownloadBytes 是 WebDAV 下载恢复文件的大小上限（512 MB）。
// 防止配置的远端返回异常超大响应时 io.ReadAll 直接耗尽内存。
const maxWebDAVDownloadBytes = 512 << 20

// WebDAVFile WebDAV 文件信息
type WebDAVFile struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	IsDir        bool      `json:"is_dir"`
}

// NewWebDAVClient 创建 WebDAV 客户端
func NewWebDAVClient(baseURL, username, password string) *WebDAVClient {
	return &WebDAVClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// longClient returns an HTTP client whose timeout only kicks in beyond the
// per-request context deadline (5 分钟), so Upload/Download 的大传输不会被
// 默认 30 秒客户端超时杀掉，同时不会出现「无任何超时」的客户端（异常挂起时
// 至多 10 分钟即可恢复）。
func (c *WebDAVClient) longClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute}
}

// Test 测试 WebDAV 连接
func (c *WebDAVClient) Test() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Depth", "0")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("服务器返回错误: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// List 列出目录下的文件
func (c *WebDAVClient) List(remotePath string) ([]WebDAVFile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullURL := c.baseURL + "/" + strings.TrimPrefix(remotePath, "/")

	// PROPFIND 请求体
	body := `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:getlastmodified/>
    <D:getcontentlength/>
    <D:resourcetype/>
  </D:prop>
</D:propfind>`

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", fullURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("服务器返回错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应（简化处理，实际需要解析 XML）
	// 这里使用简单的字符串解析作为示例
	files := parseWebDAVResponse(string(respBody), remotePath)

	return files, nil
}

// Upload 上传文件
func (c *WebDAVClient) Upload(remotePath string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fullURL := c.baseURL + "/" + strings.TrimPrefix(remotePath, "/")

	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.longClient().Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("服务器返回错误: %d %s - %s", resp.StatusCode, resp.Status, string(body))
	}

	return nil
}

// Download 下载文件
func (c *WebDAVClient) Download(remotePath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fullURL := c.baseURL + "/" + strings.TrimPrefix(remotePath, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.longClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("服务器返回错误: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxWebDAVDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取数据失败: %w", err)
	}
	if int64(len(data)) > maxWebDAVDownloadBytes {
		return nil, fmt.Errorf("下载内容超过 %d MB 上限，已中止", maxWebDAVDownloadBytes>>20)
	}

	return data, nil
}

// Delete 删除文件
func (c *WebDAVClient) Delete(remotePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullURL := c.baseURL + "/" + strings.TrimPrefix(remotePath, "/")

	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("服务器返回错误: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// parseWebDAVResponse 解析 WebDAV PROPFIND 响应。
// 先去除 XML 命名空间前缀，以兼容不同 WebDAV 服务器
// （如 D:、d:、DAV: 或无命名空间前缀）。
func parseWebDAVResponse(xmlBody, basePath string) []WebDAVFile {
	body := stripXMLNamespaces(xmlBody)

	files := make([]WebDAVFile, 0)

	responses := strings.Split(body, "<response>")
	for i, resp := range responses {
		if i == 0 {
			continue // skip preamble before the first <response>
		}

		// Extract href
		hrefStart := strings.Index(resp, "<href>")
		hrefEnd := strings.Index(resp, "</href>")
		if hrefStart == -1 || hrefEnd == -1 || hrefEnd <= hrefStart+6 {
			continue
		}
		href := resp[hrefStart+6 : hrefEnd]

		// URL-decode the href
		decodedHref, _ := url.QueryUnescape(href)

		// Extract filename from the last path segment
		trimmed := strings.TrimSuffix(decodedHref, "/")
		parts := strings.Split(trimmed, "/")
		name := parts[len(parts)-1]

		// Check if the entry is a directory (contains <collection/>)
		isDir := strings.Contains(resp, "<collection/>")

		// Skip the queried directory itself
		if decodedHref == basePath || decodedHref == basePath+"/" {
			continue
		}

		files = append(files, WebDAVFile{
			Name:         name,
			Path:         decodedHref,
			Size:         extractContentLength(resp),
			LastModified: extractLastModified(resp),
			IsDir:        isDir,
		})
	}

	return files
}

// extractContentLength parses the <getcontentlength>123</getcontentlength>
// value from a single PROPFIND <response> block. Returns 0 when absent or
// unparseable (e.g. for directories, which omit the property).
func extractContentLength(resp string) int64 {
	const open = "<getcontentlength>"
	const close = "</getcontentlength>"
	start := strings.Index(resp, open)
	if start == -1 {
		return 0
	}
	end := strings.Index(resp[start:], close)
	if end == -1 {
		return 0
	}
	value := strings.TrimSpace(resp[start+len(open) : start+end])
	var n int64
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		return 0
	}
	return n
}

// extractLastModified parses the HTTP-date in
// <getlastmodified>...</getlastmodified> from a single PROPFIND <response>
// block. Returns the zero time when absent or unparseable.
func extractLastModified(resp string) time.Time {
	const open = "<getlastmodified>"
	const close = "</getlastmodified>"
	start := strings.Index(resp, open)
	if start == -1 {
		return time.Time{}
	}
	end := strings.Index(resp[start:], close)
	if end == -1 {
		return time.Time{}
	}
	value := strings.TrimSpace(resp[start+len(open) : start+end])
	// WebDAV servers return dates in RFC1123 format (e.g.
	// "Mon, 02 Jan 2006 15:04:05 GMT").
	t, err := time.Parse(time.RFC1123, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// xmlNSRe matches XML namespace prefixes such as D:, d:, DAV: in element tags.
var xmlNSRe = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9]*:`)

// stripXMLNamespaces removes namespace prefixes from an XML body so that
// simple string-based parsing works regardless of the prefix used by the server.
func stripXMLNamespaces(body string) string {
	return xmlNSRe.ReplaceAllStringFunc(body, func(match string) string {
		if len(match) >= 2 && match[1] == '/' {
			return "</"
		}
		return "<"
	})
}
