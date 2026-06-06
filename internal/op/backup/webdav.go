package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// WebDAVFile WebDAV 文件信息
type WebDAVFile struct {
	Name         string
	Path         string
	Size         int64
	LastModified time.Time
	IsDir        bool
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

	resp, err := c.client.Do(req)
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

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("服务器返回错误: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取数据失败: %w", err)
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

// parseWebDAVResponse 解析 WebDAV PROPFIND 响应（简化版本）
func parseWebDAVResponse(xmlBody, basePath string) []WebDAVFile {
	var files []WebDAVFile

	// 简化的 XML 解析，实际应该使用 encoding/xml
	// 这里只是示例，解析 <D:response> 标签
	responses := strings.Split(xmlBody, "<D:response>")
	for i, resp := range responses {
		if i == 0 {
			continue // 跳过第一个空字符串
		}

		// 提取 href
		hrefStart := strings.Index(resp, "<D:href>")
		hrefEnd := strings.Index(resp, "</D:href>")
		if hrefStart == -1 || hrefEnd == -1 {
			continue
		}
		href := resp[hrefStart+8 : hrefEnd]

		// 解码 URL
		decodedHref, _ := url.QueryUnescape(href)

		// 提取文件名
		parts := strings.Split(strings.TrimSuffix(decodedHref, "/"), "/")
		name := parts[len(parts)-1]

		// 检查是否是目录
		isDir := strings.Contains(resp, "<D:collection/>")

		// 跳过当前目录本身
		if decodedHref == basePath || decodedHref == basePath+"/" {
			continue
		}

		files = append(files, WebDAVFile{
			Name:  name,
			Path:  decodedHref,
			IsDir: isDir,
			// Size 和 LastModified 需要进一步解析，这里简化处理
		})
	}

	return files
}
