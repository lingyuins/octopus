// Package nodetransport implements a bounded, non-replaying reverse model tunnel.
package nodetransport

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

const BaseURL = "https://node.octopus.invalid/v1"

// MaxModelsResponseBytes bounds the complete, non-streaming model list.
const MaxModelsResponseBytes = 4 << 20

const (
	protocol     = "octopus-node-v1"
	maxChunk     = 32 << 10
	maxHeader    = 16 << 10
	maxRequest   = 4 << 20
	maxStreams   = 32
	queueSize    = 64
	// defaultStreamQueue 是每条流在内存里缓冲的响应帧数默认上限（8 × 32KiB = 256KiB）。
	// 推理模型"长思考后瞬时爆发输出"的模式很容易打满它，可由
	// node_transport.stream_queue_frames / OCTOPUS_NODE_TRANSPORT_STREAM_QUEUE_FRAMES 覆盖。
	defaultStreamQueue = 8
	writeTimeout = 10 * time.Second
)
const (
	frameHello byte = iota + 1
	frameRequest
	frameRequestData
	frameRequestEnd
	frameResponse
	frameResponseData
	frameComplete
	frameError
	frameCancel
)

var (
	ErrNodeOffline         = errors.New("node offline: request not dispatched")
	ErrNodeBusy            = errors.New("node concurrency limit: request not dispatched")
	ErrInvalidTarget       = errors.New("invalid node model target")
	ErrSlowConsumer        = errors.New("node response consumer too slow")
	ErrLocalUnavailable    = errors.New("local Magpie gateway unavailable")
	ErrLocalAuthentication = errors.New("local Magpie authentication failed")
	ErrLocalHTTP           = errors.New("local Magpie returned an unexpected HTTP status")
	ErrInvalidModels       = errors.New("local Magpie returned an invalid model list")
)

type ExecutionUncertainError struct{ Err error }

func (e *ExecutionUncertainError) Error() string { return "node execution uncertain: " + e.Err.Error() }
func (e *ExecutionUncertainError) Unwrap() error { return e.Err }
func IsExecutionUncertain(err error) bool        { var e *ExecutionUncertainError; return errors.As(err, &e) }

// streamQueueFrames 返回当前生效的每流缓冲帧数（默认 8）。
// 优先级：DB setting（设置页可改，重启生效）> config.json/env。
func streamQueueFrames() int {
	if v, err := setting.GetString(model.SettingKeyNodeTransportStreamQueueFrames); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	if n := conf.AppConfig.NodeTransport.StreamQueueFrames; n > 0 {
		return n
	}
	return defaultStreamQueue
}

// consumerWaitTimeout 返回消费队列满时等待消费端追上的宽限。
// 0 表示立即判慢消费（旧行为）。优先级：DB setting > config.json/env。
func consumerWaitTimeout() time.Duration {
	if v, err := setting.GetString(model.SettingKeyNodeTransportConsumerWaitTimeout); err == nil {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return conf.AppConfig.NodeTransport.ConsumerWaitTimeout
}

// ValidateChannelTarget validates a channel base URL, not an arbitrary upstream URL.
func ValidateChannelTarget(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != "node.octopus.invalid" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "/v1" && u.Path != "/v1/") {
		return ErrInvalidTarget
	}
	return nil
}
func allowedTarget(method, path string) bool {
	if method == http.MethodGet && path == "/v1/models" {
		return true
	}
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/embeddings":
		return true
	}
	return false
}
func safeRequestURL(u *url.URL, method string) bool {
	return u != nil && u.Scheme == "https" && u.Host == "node.octopus.invalid" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" && allowedTarget(method, u.Path)
}
func filterHeaders(src http.Header, response bool) http.Header {
	dst := make(http.Header)
	for k, values := range src {
		name := strings.ToLower(k)
		ok := name == "content-type"
		if response {
			ok = ok || name == "cache-control" || name == "retry-after" || name == "request-id" || name == "x-request-id"
		} else {
			ok = ok || name == "accept" || name == "anthropic-version" || name == "anthropic-beta" || name == "openai-beta"
		}
		if ok {
			dst[http.CanonicalHeaderKey(k)] = append([]string(nil), values...)
		}
	}
	return dst
}
func boundedHeaders(h http.Header) bool {
	b, err := json.Marshal(h)
	if err != nil || len(b) > maxHeader {
		return false
	}
	for k, vv := range h {
		for _, v := range vv {
			if strings.ContainsAny(k+v, "\r\n\x00") {
				return false
			}
		}
	}
	return true
}

type requestMeta struct {
	Method   string      `json:"method"`
	Path     string      `json:"path"`
	Header   http.Header `json:"header"`
	Deadline int64       `json:"deadline"`
}
type responseMeta struct {
	Status int         `json:"status"`
	Header http.Header `json:"header"`
}
type hello struct {
	Version        int `json:"version"`
	MaxConcurrency int `json:"max_concurrency"`
}
type frame struct {
	kind byte
	id   uint64
	data []byte
}

func (f frame) encode() []byte {
	b := make([]byte, 9+len(f.data))
	b[0] = f.kind
	binary.BigEndian.PutUint64(b[1:9], f.id)
	copy(b[9:], f.data)
	return b
}
func readFrame(ctx context.Context, c *websocket.Conn) (frame, error) {
	typ, b, err := c.Read(ctx)
	if err != nil {
		return frame{}, err
	}
	if typ != websocket.MessageBinary || len(b) < 9 || len(b) > maxChunk+9 {
		return frame{}, errors.New("invalid tunnel frame")
	}
	return frame{b[0], binary.BigEndian.Uint64(b[1:9]), b[9:]}, nil
}
func jsonFrame(kind byte, id uint64, v interface{}) (frame, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return frame{}, err
	}
	if len(b) > maxHeader {
		return frame{}, errors.New("tunnel metadata too large")
	}
	return frame{kind, id, b}, nil
}

type outgoing struct {
	frame frame
	ctx   context.Context
	sent  *atomic.Bool
}
type wire struct {
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	data     chan outgoing
	control  chan outgoing
	once     sync.Once
	lastSeen atomic.Int64
}

func newWire(ctx context.Context, c *websocket.Conn) *wire {
	ctx, cancel := context.WithCancel(ctx)
	c.SetReadLimit(maxChunk + 9)
	w := &wire{conn: c, ctx: ctx, cancel: cancel, data: make(chan outgoing, queueSize), control: make(chan outgoing, queueSize)}
	go w.writer()
	go w.heartbeat()
	return w
}
func (w *wire) close() { w.once.Do(func() { w.cancel(); _ = w.conn.CloseNow() }) }
func (w *wire) send(ctx context.Context, f frame, sent *atomic.Bool, control bool) error {
	q := w.data
	if control {
		q = w.control
	}
	select {
	case <-w.ctx.Done():
		return ErrNodeOffline
	default:
	}
	writeCtx := w.ctx
	if f.kind == frameRequest || f.kind == frameRequestData || f.kind == frameRequestEnd {
		writeCtx = ctx
	}
	select {
	case q <- outgoing{f, writeCtx, sent}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-w.ctx.Done():
		return ErrNodeOffline
	}
}

// Control frames never wait for a slow data consumer. Saturation closes the
// connection rather than retaining an unbounded cancellation backlog.
func (w *wire) cancelStream(id uint64) {
	select {
	case w.control <- outgoing{frame: frame{kind: frameCancel, id: id}, ctx: w.ctx}:
	default:
		w.close()
	}
}
func (w *wire) writer() {
	defer w.close()
	for {
		var o outgoing
		select {
		case <-w.ctx.Done():
			return
		case o = <-w.control:
		default:
			select {
			case <-w.ctx.Done():
				return
			case o = <-w.control:
			case o = <-w.data:
			}
		}
		if o.ctx.Err() != nil {
			continue
		}
		// Mark before the first Write: a failed write cannot prove that the peer
		// did not receive or execute a request. Never replay it on a new session.
		if o.sent != nil {
			o.sent.Store(true)
		}
		ctx, cancel := context.WithTimeout(w.ctx, writeTimeout)
		err := w.conn.Write(ctx, websocket.MessageBinary, o.frame.encode())
		cancel()
		if err != nil {
			return
		}
	}
}
func (w *wire) heartbeat() {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
			err := w.conn.Ping(ctx)
			cancel()
			if err != nil {
				w.close()
				return
			}
			w.lastSeen.Store(time.Now().UnixMilli())
		}
	}
}
func protocolError(s string) error { return fmt.Errorf("node protocol: %s", s) }
