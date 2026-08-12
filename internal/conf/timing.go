package conf

import "time"

const SSEHeartbeatInterval = 15 * time.Second

// HTTP 服务器基础超时（Slowloris 防护）。只约束读取侧与空闲连接，
// 不设置 WriteTimeout——流式（SSE）响应需要长时间写。
const (
	ServerReadHeaderTimeout = 10 * time.Second
	ServerReadTimeout       = 60 * time.Second
	ServerIdleTimeout       = 120 * time.Second
)
