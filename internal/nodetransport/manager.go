package nodetransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

var DefaultManager = NewManager()

var ErrFeatureDisabled = errors.New("magpie nodes feature is disabled")

// FeatureEnabledFunc is wired at startup; standalone transport tests leave it nil.
var FeatureEnabledFunc func() bool

func featureEnabled() bool { return FeatureEnabledFunc == nil || FeatureEnabledFunc() }

type AcceptOptions struct {
	MaxConcurrency int
	// Validate runs under the registration mutex AFTER the WebSocket handshake.
	// Query current credentials/epoch from DB here; do not hold an operation lock
	// or call Manager methods. Revoke by committing DB state then Disconnect.
	Validate func() bool
}
type Status struct {
	Online           bool       `json:"online"`
	ConnectedAt      time.Time  `json:"connected_at"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	ConnectorVersion string     `json:"connector_version"`
	InFlight         int        `json:"in_flight"`
	MaxConcurrency   int        `json:"max_concurrency"`
	LastError        string     `json:"last_error"`
}
type Manager struct {
	mu       sync.Mutex
	sessions map[int]*serverSession
	history  map[int]Status
	closed   bool
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[int]*serverSession), history: make(map[int]Status)}
}

func (m *Manager) rememberOffline(nodeID int, s *serverSession, reason string) {
	if len(m.history) >= 1024 {
		var oldestID int
		var oldest time.Time
		for id, status := range m.history {
			if oldest.IsZero() || (status.LastSeenAt != nil && status.LastSeenAt.Before(oldest)) {
				oldestID = id
				if status.LastSeenAt != nil {
					oldest = *status.LastSeenAt
				}
			}
		}
		delete(m.history, oldestID)
	}
	seen := time.UnixMilli(s.lastSeen.Load()).UTC()
	m.history[nodeID] = Status{ConnectedAt: s.connected, LastSeenAt: &seen, ConnectorVersion: protocol, MaxConcurrency: s.limit, LastError: reason}
}
func clampConcurrency(n int) int {
	if n <= 0 {
		return 4
	}
	if n > maxStreams {
		return maxStreams
	}
	return n
}
func (m *Manager) Accept(ctx context.Context, nodeID int, w http.ResponseWriter, r *http.Request, opts AcceptOptions) error {
	if !featureEnabled() {
		http.Error(w, ErrFeatureDisabled.Error(), http.StatusForbidden)
		return ErrFeatureDisabled
	}
	if nodeID <= 0 || opts.Validate == nil {
		http.Error(w, "node authorization required", http.StatusForbidden)
		return errors.New("missing node validation")
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{protocol}})
	if err != nil {
		return err
	}
	if c.Subprotocol() != protocol {
		_ = c.CloseNow()
		return protocolError("subprotocol required")
	}
	// A pending handshake is bounded independently of the long-lived session.
	hctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	c.SetReadLimit(maxChunk + 9)
	f, err := readFrame(hctx, c)
	cancel()
	var h hello
	if err != nil || f.kind != frameHello || f.id != 0 || json.Unmarshal(f.data, &h) != nil || h.Version != 1 || h.MaxConcurrency < 1 || h.MaxConcurrency > maxStreams {
		_ = c.CloseNow()
		return protocolError("invalid hello")
	}
	limit := clampConcurrency(opts.MaxConcurrency)
	if h.MaxConcurrency < limit {
		limit = h.MaxConcurrency
	}
	m.mu.Lock()
	if m.closed || !featureEnabled() || !opts.Validate() {
		m.mu.Unlock()
		_ = c.CloseNow()
		return errors.New("node registration revoked")
	}
	s := &serverSession{wire: newWire(ctx, c), streams: make(map[uint64]*responseStream), capacity: make(chan struct{}, limit), limit: limit, connected: time.Now().UTC()}
	s.lastSeen.Store(time.Now().UnixMilli())
	old := m.sessions[nodeID]
	m.sessions[nodeID] = s
	delete(m.history, nodeID)
	m.mu.Unlock()
	if old != nil {
		old.close()
	}
	err = s.readLoop()
	s.close()
	m.mu.Lock()
	if m.sessions[nodeID] == s {
		delete(m.sessions, nodeID)
		m.rememberOffline(nodeID, s, "node connection closed")
	}
	m.mu.Unlock()
	return err
}
func (m *Manager) Disconnect(nodeID int) {
	m.mu.Lock()
	s := m.sessions[nodeID]
	delete(m.sessions, nodeID)
	if s != nil {
		m.rememberOffline(nodeID, s, "node disconnected")
	}
	m.mu.Unlock()
	if s != nil {
		s.close()
	}
}

// DisconnectAll revokes current sessions but permits new connections after re-enabling.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	ss := m.sessions
	m.sessions = make(map[int]*serverSession)
	for id, s := range ss {
		m.rememberOffline(id, s, "node feature disabled")
	}
	m.mu.Unlock()
	for _, s := range ss {
		s.close()
	}
}

func (m *Manager) Revoke(nodeID int) { m.Disconnect(nodeID) }
func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	ss := m.sessions
	m.sessions = make(map[int]*serverSession)
	m.mu.Unlock()
	for _, s := range ss {
		s.close()
	}
	return nil
}
func (m *Manager) Status(nodeID int) Status {
	m.mu.Lock()
	s := m.sessions[nodeID]
	previous := m.history[nodeID]
	m.mu.Unlock()
	if s == nil {
		return previous
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return Status{}
	}
	seen := time.UnixMilli(s.lastSeen.Load()).UTC()
	return Status{Online: true, ConnectedAt: s.connected, LastSeenAt: &seen, ConnectorVersion: protocol, InFlight: len(s.capacity), MaxConcurrency: s.limit}
}
func (m *Manager) Snapshot(nodeID int) Status { return m.Status(nodeID) }
func (m *Manager) Client(nodeID int, timeout time.Duration) *http.Client {
	return &http.Client{Transport: &nodeTransport{manager: m, nodeID: nodeID, timeout: timeout}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func Client(nodeID int, timeout time.Duration) *http.Client {
	return DefaultManager.Client(nodeID, timeout)
}

type serverSession struct {
	*wire
	mu         sync.Mutex
	streams    map[uint64]*responseStream
	dispatchMu sync.Mutex
	next       atomic.Uint64
	limit      int
	capacity   chan struct{}
	connected  time.Time
}

func (s *serverSession) close() {
	s.wire.close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.streams {
		st.finish(ErrNodeOffline)
		st.stopUpload()
		delete(s.streams, id)
	}
}
func (s *serverSession) remove(id uint64, err error, cancel bool) {
	s.mu.Lock()
	st := s.streams[id]
	delete(s.streams, id)
	s.mu.Unlock()
	if st != nil {
		st.finish(err)
		st.stopUpload()
		if cancel {
			s.cancelStream(id)
		}
	}
}
func (s *serverSession) readLoop() error {
	for {
		f, err := readFrame(s.ctx, s.conn)
		if err != nil {
			return err
		}
		s.lastSeen.Store(time.Now().UnixMilli())
		s.mu.Lock()
		st := s.streams[f.id]
		s.mu.Unlock()
		if st == nil { // Late frames after cancellation are deliberately discarded.
			if f.kind < frameResponse || f.kind > frameError {
				return protocolError("unexpected frame")
			}
			continue
		}
		select {
		case <-st.done:
			continue
		default:
		}
		switch f.kind {
		case frameResponse:
			var meta responseMeta
			if st.headers || len(f.data) > maxHeader || json.Unmarshal(f.data, &meta) != nil || meta.Status < 200 || meta.Status > 599 || !boundedHeaders(meta.Header) {
				return protocolError("invalid response headers")
			}
			meta.Header = filterHeaders(meta.Header, true)
			st.headers = true
			st.meta <- meta
		case frameResponseData:
			if !st.headers {
				return protocolError("data before response")
			}
			if len(f.data) == 0 {
				continue
			}
			if st.models {
				if len(f.data) > MaxModelsResponseBytes-len(st.modelsBody) {
					st.modelsBody = nil
					s.remove(f.id, ErrInvalidModels, true)
					continue
				}
				// Models are finite metadata. Buffer by bytes without making the
				// shared reader wait for this stream's consumer or frame count.
				if size := len(st.modelsBody) + len(f.data); size > cap(st.modelsBody) {
					capacity := min(MaxModelsResponseBytes, max(size, 2*cap(st.modelsBody)))
					body := make([]byte, len(st.modelsBody), capacity)
					copy(body, st.modelsBody)
					st.modelsBody = body
				}
				st.modelsBody = append(st.modelsBody, f.data...)
				continue
			}
			// 缓冲满时先给消费端一个可配置的宽限（默认 0 = 立即判慢消费，
			// 兼容旧行为），把瞬时抖动与消费端真的卡死区分开。
			if !s.deliverChunk(st, f.data) {
				s.remove(f.id, ErrSlowConsumer, true)
			}
		case frameComplete:
			if !st.headers || len(f.data) != 0 {
				return protocolError("invalid completion")
			}
			if st.models && len(st.modelsBody) > 0 {
				st.chunks <- st.modelsBody
				st.modelsBody = nil
			}
			// Completion ends execution, not ownership of queued body bytes.
			// Retain registration and capacity until the consumer reaches EOF/Close.
			st.finish(nil)
			st.stopUpload()
		case frameError:
			// Reflect only fixed diagnostic codes, never arbitrary peer text.
			var failure error
			switch string(f.data) {
			case "deadline":
				failure = context.DeadlineExceeded
			case "local_unavailable":
				failure = ErrLocalUnavailable
			case "local_authentication":
				failure = ErrLocalAuthentication
			case "local_http":
				failure = ErrLocalHTTP
			case "invalid_models":
				failure = ErrInvalidModels
			default:
				failure = errors.New("connector upstream request failed")
			}
			s.remove(f.id, failure, false)
		default:
			return protocolError("unexpected frame")
		}
	}
}

type responseStream struct {
	meta       chan responseMeta
	chunks     chan []byte
	done       chan struct{}
	once       sync.Once
	sent       atomic.Bool
	headers    bool   // readLoop owns this field
	models     bool   // fixed GET /v1/models, buffered until completion
	modelsBody []byte // readLoop owns this field
	err        error  // published by closing done
	stopUpload context.CancelFunc
}

func newResponseStream() *responseStream {
	return &responseStream{meta: make(chan responseMeta, 1), chunks: make(chan []byte, streamQueueFrames()), done: make(chan struct{})}
}
func (s *responseStream) finish(err error) { s.once.Do(func() { s.err = err; close(s.done) }) }
func (s *responseStream) executionError(err error) error {
	if err != nil && s.sent.Load() {
		return &ExecutionUncertainError{err}
	}
	return err
}

type nodeTransport struct {
	manager *Manager
	nodeID  int
	timeout time.Duration
}

// deliverChunk 把一帧响应数据放进该流的消费队列。队列满时先按
// consumerWaitTimeout 给消费端一个追上的宽限（默认 0 = 立即判慢消费，
// 兼容旧行为），宽限用尽才返回 false 由调用方终止流。
func (s *serverSession) deliverChunk(st *responseStream, data []byte) bool {
	select {
	case st.chunks <- data:
		return true
	default:
	}
	wait := consumerWaitTimeout()
	if wait <= 0 {
		return false
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case st.chunks <- data:
		return true
	case <-timer.C:
		return false
	}
}

// Keep the timeout inside the transport: net/http.Client.Timeout replaces a
// RoundTripper error with an unwrappable timeoutError, losing execution safety.
func (t *nodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	timeout := t.timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	resp, err := t.roundTrip(req.Clone(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &deadlineBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type deadlineBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	eof    atomic.Bool
}

func (b *deadlineBody) Read(p []byte) (int, error) {
	if b.eof.Load() {
		return 0, io.EOF
	}
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		if err == io.EOF {
			b.eof.Store(true)
		}
		b.cancel()
	}
	return n, err
}
func (b *deadlineBody) Close() error { err := b.ReadCloser.Close(); b.cancel(); return err }
func (t *nodeTransport) roundTrip(req *http.Request) (*http.Response, error) {
	if !featureEnabled() {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, ErrFeatureDisabled
	}
	if !safeRequestURL(req.URL, req.Method) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, ErrInvalidTarget
	}
	h := filterHeaders(req.Header, false)
	if !boundedHeaders(h) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, errors.New("request headers too large")
	}
	t.manager.mu.Lock()
	s := t.manager.sessions[t.nodeID]
	t.manager.mu.Unlock()
	if s == nil || s.ctx.Err() != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, ErrNodeOffline
	}
	var releaseOnce sync.Once
	releaseCapacity := func() { releaseOnce.Do(func() { <-s.capacity }) }
	bodyOwnsCapacity := false
	select {
	case s.capacity <- struct{}{}:
		defer func() {
			if !bodyOwnsCapacity {
				releaseCapacity()
			}
		}()
	default:
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, ErrNodeBusy
	}
	var body []byte
	if req.Body != nil {
		stop := context.AfterFunc(req.Context(), func() { _ = req.Body.Close() })
		var err error
		body, err = io.ReadAll(io.LimitReader(req.Body, maxRequest+1))
		stop()
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(body) > maxRequest {
			return nil, errors.New("node request exceeds limit")
		}
	}
	if req.Context().Err() != nil {
		return nil, req.Context().Err()
	}
	if s.ctx.Err() != nil {
		return nil, ErrNodeOffline
	}
	s.dispatchMu.Lock()
	if !featureEnabled() {
		s.dispatchMu.Unlock()
		return nil, ErrFeatureDisabled
	}
	st := newResponseStream()
	st.models = req.Method == http.MethodGet && req.URL.Path == "/v1/models"
	id := s.next.Add(1)
	uploadCtx, stopUpload := context.WithCancel(req.Context())
	st.stopUpload = stopUpload
	s.mu.Lock()
	if len(s.streams) >= s.limit {
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		stopUpload()
		return nil, ErrNodeBusy
	}
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		s.dispatchMu.Unlock()
		stopUpload()
		return nil, ErrNodeOffline
	}
	s.streams[id] = st
	s.mu.Unlock()
	deadline := time.Now().Add(5 * time.Minute)
	if d, ok := req.Context().Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	meta := requestMeta{Method: req.Method, Path: req.URL.Path, Header: h, Deadline: deadline.UnixMilli()}
	f, err := jsonFrame(frameRequest, id, meta)
	if err != nil {
		s.remove(id, err, false)
		s.dispatchMu.Unlock()
		return nil, err
	}
	opCtx, stopDeadline := context.WithDeadline(req.Context(), deadline)
	// Cancellation is tied to the operation context or Body.Close, not to an
	// unrelated downstream TCP connection (stream sessions can outlive it).
	stopAfter := context.AfterFunc(opCtx, func() { s.remove(id, opCtx.Err(), true) })
	stopContext := func() bool { stopped := stopAfter(); stopDeadline(); return stopped }
	st.sent.Store(true)
	if e := s.send(uploadCtx, f, &st.sent, false); e != nil {
		s.remove(id, e, false)
		s.dispatchMu.Unlock()
		stopContext()
		return nil, st.executionError(e)
	}
	s.dispatchMu.Unlock()
	go func() {
		for len(body) > 0 {
			n := len(body)
			if n > maxChunk {
				n = maxChunk
			}
			if e := s.send(uploadCtx, frame{frameRequestData, id, body[:n]}, nil, false); e != nil {
				s.remove(id, e, true)
				return
			}
			body = body[n:]
		}
		if e := s.send(uploadCtx, frame{kind: frameRequestEnd, id: id}, nil, false); e != nil {
			s.remove(id, e, true)
		}
	}()
	makeResponse := func(meta responseMeta) *http.Response {
		bodyOwnsCapacity = true
		return &http.Response{StatusCode: meta.Status, Status: http.StatusText(meta.Status), Header: meta.Header, Body: &responseBody{st: st, session: s, id: id, ctx: req.Context(), stop: stopContext, stopUpload: stopUpload, releaseCapacity: releaseCapacity}, ContentLength: -1, Request: req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}
	}
	select {
	case meta := <-st.meta:
		return makeResponse(meta), nil
	case <-st.done:
		select {
		case meta := <-st.meta:
			return makeResponse(meta), nil
		default:
		}
		stopContext()
		stopUpload()
		err := st.err
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, st.executionError(err)
	case <-req.Context().Done():
		s.remove(id, req.Context().Err(), true)
		stopContext()
		stopUpload()
		return nil, st.executionError(req.Context().Err())
	}
}

type responseBody struct {
	st              *responseStream
	session         *serverSession
	id              uint64
	ctx             context.Context
	stop            func() bool
	stopUpload      context.CancelFunc
	pending         []byte
	closed          atomic.Bool
	releaseOnce     sync.Once
	releaseCapacity func()
}

func (b *responseBody) release() {
	b.releaseOnce.Do(func() { b.stop(); b.stopUpload(); b.session.remove(b.id, context.Canceled, true); b.releaseCapacity() })
}

func (b *responseBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	if err := b.ctx.Err(); err != nil {
		b.release()
		return 0, b.st.executionError(err)
	}
	for len(b.pending) == 0 {
		select {
		case b.pending = <-b.st.chunks:
		default:
		}
		if len(b.pending) > 0 {
			break
		}
		select {
		case b.pending = <-b.st.chunks:
		case <-b.st.done:
			select {
			case b.pending = <-b.st.chunks:
			default:
			}
			if len(b.pending) > 0 {
				break
			}
			b.release()
			if b.st.err != nil {
				return 0, b.st.executionError(b.st.err)
			}
			return 0, io.EOF
		case <-b.ctx.Done():
			b.release()
			return 0, b.st.executionError(b.ctx.Err())
		}
	}
	n := copy(p, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}
func (b *responseBody) Close() error {
	if !b.closed.Swap(true) {
		b.release()
	}
	return nil
}
