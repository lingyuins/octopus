package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"golang.org/x/net/proxy"
)

const (
	// shortTaskTimeout 后台任务使用的短超时时间（延迟探测、模型同步）
	shortTaskTimeout = 30 * time.Second
)

var (
	systemDirectClient      *http.Client
	systemProxyClient       *http.Client
	systemProxyURL          string
	shortTimeoutDirectClient *http.Client
	shortTimeoutProxyClient  *http.Client
	clientLock              sync.RWMutex
)

// GetHTTPClientSystemProxy returns a cached http.Client.
// - useProxy=false: bypass proxy
// - useProxy=true: use proxy settings from system/app settings (setting key: proxy_url)
func GetHTTPClientSystemProxy(useProxy bool) (*http.Client, error) {
	if useProxy {
		currentProxyURL, err := setting.GetString(model.SettingKeyProxyURL)
		if err != nil {
			return nil, err
		}
		if currentProxyURL == "" {
			return nil, fmt.Errorf("proxy url is empty")
		}

		clientLock.RLock()
		if systemProxyClient != nil && systemProxyURL == currentProxyURL {
			clientLock.RUnlock()
			return systemProxyClient, nil
		}
		clientLock.RUnlock()

		clientLock.Lock()
		defer clientLock.Unlock()

		// Re-check after acquiring write lock.
		if systemProxyClient != nil && systemProxyURL == currentProxyURL {
			return systemProxyClient, nil
		}

		client, err := newHTTPClientCustomProxy(currentProxyURL)
		if err != nil {
			return nil, err
		}
		systemProxyClient = client
		systemProxyURL = currentProxyURL
		return systemProxyClient, nil
	}

	clientLock.RLock()
	if !useProxy && systemDirectClient != nil {
		clientLock.RUnlock()
		return systemDirectClient, nil
	}
	clientLock.RUnlock()

	clientLock.Lock()
	defer clientLock.Unlock()

	if systemDirectClient != nil {
		return systemDirectClient, nil
	}
	client, err := newHTTPClientNoProxy()
	if err != nil {
		return nil, err
	}
	systemDirectClient = client
	return systemDirectClient, nil
}

// GetHTTPClientCustomProxy returns a NEW http.Client every time (no reuse).
// proxyURL supports: http, https, socks, socks5
func GetHTTPClientCustomProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return nil, fmt.Errorf("proxy url is empty")
	}
	return newHTTPClientCustomProxy(proxyURL)
}

// GetHTTPClientShortTimeout returns a cached http.Client with short timeout (30s).
// Used for background tasks like delay probing and model syncing to avoid
// goroutine/connection accumulation when endpoints are unreachable.
// - useProxy=false: bypass proxy
// - useProxy=true: use proxy settings from system/app settings
func GetHTTPClientShortTimeout(useProxy bool) (*http.Client, error) {
	if useProxy {
		currentProxyURL, err := setting.GetString(model.SettingKeyProxyURL)
		if err != nil {
			return nil, err
		}
		if currentProxyURL == "" {
			return nil, fmt.Errorf("proxy url is empty")
		}

		clientLock.RLock()
		if shortTimeoutProxyClient != nil && systemProxyURL == currentProxyURL {
			clientLock.RUnlock()
			return shortTimeoutProxyClient, nil
		}
		clientLock.RUnlock()

		clientLock.Lock()
		defer clientLock.Unlock()

		if shortTimeoutProxyClient != nil && systemProxyURL == currentProxyURL {
			return shortTimeoutProxyClient, nil
		}

		client, err := newHTTPClientCustomProxyWithTimeout(currentProxyURL, shortTaskTimeout)
		if err != nil {
			return nil, err
		}
		shortTimeoutProxyClient = client
		return shortTimeoutProxyClient, nil
	}

	clientLock.RLock()
	if shortTimeoutDirectClient != nil {
		clientLock.RUnlock()
		return shortTimeoutDirectClient, nil
	}
	clientLock.RUnlock()

	clientLock.Lock()
	defer clientLock.Unlock()

	if shortTimeoutDirectClient != nil {
		return shortTimeoutDirectClient, nil
	}
	client, err := newHTTPClientNoProxyWithTimeout(shortTaskTimeout)
	if err != nil {
		return nil, err
	}
	shortTimeoutDirectClient = client
	return shortTimeoutDirectClient, nil
}

// GetHTTPClientCustomProxyWithTimeout returns a NEW http.Client with custom timeout.
// proxyURL supports: http, https, socks, socks5
func GetHTTPClientCustomProxyWithTimeout(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if proxyURL == "" {
		return nil, fmt.Errorf("proxy url is empty")
	}
	return newHTTPClientCustomProxyWithTimeout(proxyURL, timeout)
}

func clonedDefaultTransport() (*http.Transport, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default transport is not *http.Transport")
	}
	return transport.Clone(), nil
}

func newHTTPClientNoProxy() (*http.Client, error) {
	return newHTTPClientNoProxyWithTimeout(600 * time.Second)
}

func newHTTPClientNoProxyWithTimeout(timeout time.Duration) (*http.Client, error) {
	cloned, err := clonedDefaultTransport()
	if err != nil {
		return nil, err
	}
	cloned.Proxy = nil
	return &http.Client{Transport: cloned, Timeout: timeout}, nil
}

func newHTTPClientCustomProxy(proxyURLStr string) (*http.Client, error) {
	return newHTTPClientCustomProxyWithTimeout(proxyURLStr, 600*time.Second)
}

func newHTTPClientCustomProxyWithTimeout(proxyURLStr string, timeout time.Duration) (*http.Client, error) {
	cloned, err := clonedDefaultTransport()
	if err != nil {
		return nil, err
	}

	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}

	switch proxyURL.Scheme {
	case "http", "https":
		cloned.Proxy = http.ProxyURL(proxyURL)
	case "socks", "socks5":
		socksDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("invalid socks proxy: %w", err)
		}
		cloned.Proxy = nil
		cloned.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socksDialer.Dial(network, addr)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
	}

	return &http.Client{Transport: cloned, Timeout: timeout}, nil
}
