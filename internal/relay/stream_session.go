package relay

import (
	"bytes"
	"context"

	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/utils/log"
)

var (
	errRelayConversationBusy    = errors.New("conversation already has an active generation")
	errRelayReplayWindowExpired = errors.New("relay stream replay window expired")

	// errRelayStreamSessionEvicted 表示会话因为会话总数超过硬上限而被强制驱逐。
	// 只会发生在内存保护兜底路径上（见 enforceSessionLimitLocked）。
	errRelayStreamSessionEvicted = errors.New("relay stream session evicted: too many concurrent sessions")

	// errRelayStreamClientGone 表示客户端断连后上游在宽限期内仍未结束，会话被
	// 强制结束以释放缓冲（见 relayStreamClientGoneGrace）。
	errRelayStreamClientGone = errors.New("relay stream session closed: client gone and upstream exceeded grace period")

	overrideStreamSessionTTL         *time.Duration
	overrideStreamSessionMaxEvents   *int
	overrideStreamSessionMaxBytes    *int
	overrideStreamSessionMaxSessions *int
)

const (
	// relayStreamDoneRetention 是会话完成（Finish）后其 replay 缓冲区的保留时长。
	// 完成会话的缓冲区仅用于断线重连重放——客户端在生成结束后短时间内重连读取
	// 已生成内容。这个窗口远短于会话 TTL：之前完成会话会连同最多 16MB 的缓冲区
	// 一起驻留整个 TTL（默认 30 分钟），高并发流式下常驻内存累积到数百 MB 并
	// 触发 swap（见 issue #46）。改为最多保留 2 分钟即清理，把大缓冲的驻留时间
	// 压缩一个数量级，同时保留断线重连重放语义。
	relayStreamDoneRetention = 2 * time.Minute

	// relayStreamMaxSessions 是全局会话 map 的默认硬上限，可由设置
	// stream_session_max_sessions 覆盖。超过时优先驱逐最旧的已完成会话；仍然
	// 超限时按最旧优先驱逐活跃会话（见 relayStreamActiveEvictFactor）。
	//
	// 默认值从 4096 下调到 512：4096 × 单会话 4MB 缓冲的理论上限是 16GB，对
	// 自建部署（常见 8~16GB 内存）来说等于没有上限。512 × 4MB = 2GB，仍然远
	// 大于正常并发所需，同时把最坏情况压到可控范围（见 issue #196 的 OOM）。
	relayStreamMaxSessions = 512

	// relayStreamActiveEvictFactor 是活跃会话开始被驱逐的宽限倍数：会话总数超过
	// maxSessions × factor 时，最旧的活跃会话也会被驱逐。
	//
	// 之前只驱逐 done 会话，活跃会话「永不驱逐」——注释写的是「活跃会话的 buffer
	// 受单会话上限约束，且会在 Finish 后释放」，但客户端中途断连时会话会一直保持
	// 活跃直到上游结束，于是 map 可以无界增长到把内存吃光（issue #196：16GB 主机
	// 上 RSS 涨到 13.8GB 被内核 OOM killer 杀掉两次）。留一倍宽限是为了尽量不打断
	// 正在进行的生成，只在确实要 OOM 的方向上才动活跃会话。
	relayStreamActiveEvictFactor = 2

	// relayStreamClientGoneGrace 是客户端断连后允许上游继续生成的宽限时长。
	// 到期仍未结束则强制 Finish 该会话并停止读取上游，把「断连会话」占用的
	// 缓冲内存窗口限制在这个量级内，而不是跟随上游的完整生成时长。
	//
	// 保留一段宽限而不是立刻结束，是为了不破坏断线重连重放语义：客户端网络抖动后
	// 重连仍能拿到这段时间内继续生成的内容。
	relayStreamClientGoneGrace = 30 * time.Second

	// relayStreamClientGoneCheckInterval 是断连宽限期的轮询间隔。取远小于宽限期
	// 的值，保证宽限期一到就能及时收尾；同时不必太小——它只在有 stream session
	// 的流式请求上跑一个 ticker。
	relayStreamClientGoneCheckInterval = 5 * time.Second
)

// getStreamSessionMaxSessions 返回全局会话 map 的硬上限。
// 设置为 0 或负数表示不限制（与旧行为一致，仅供明确知晓风险的部署使用）。
func getStreamSessionMaxSessions() int {
	if overrideStreamSessionMaxSessions != nil {
		return *overrideStreamSessionMaxSessions
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyStreamSessionMaxSessions); err == nil && v >= 0 {
		return v
	}
	return relayStreamMaxSessions
}

func getStreamSessionTTL() time.Duration {
	if overrideStreamSessionTTL != nil {
		return *overrideStreamSessionTTL
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyStreamSessionTTLMinutes); err == nil && v > 0 {
		return time.Duration(v) * time.Minute
	}
	return 30 * time.Minute
}

func getStreamSessionMaxEvents() int {
	if overrideStreamSessionMaxEvents != nil {
		return *overrideStreamSessionMaxEvents
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyStreamSessionMaxEvents); err == nil && v > 0 {
		return v
	}
	return 4096
}

func getStreamSessionMaxBytes() int {
	if overrideStreamSessionMaxBytes != nil {
		return *overrideStreamSessionMaxBytes
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyStreamSessionMaxBytesMB); err == nil && v > 0 {
		return v << 20
	}
	return 4 << 20
}

type relayStreamEvent struct {
	Sequence int64
	Payload  []byte
}

type relayStreamSession struct {
	store             *relayStreamSessionStore
	key               string
	conversationID    string
	conversationScope string
	requestHash       uint64
	createdAt         time.Time

	mu               sync.RWMutex
	updatedAt        time.Time
	done             bool
	err              error
	clientGoneAt     time.Time // 客户端断连时刻；零值表示客户端仍在
	clientGoneActive bool      // 客户端断连后是否已开始强制结束计时（见 MarkClientGone）
	nextSeq          int64
	droppedBeforeSeq int64
	bufferBytes      int
	events           []relayStreamEvent
	subscribers      map[chan struct{}]struct{}
}

type relayStreamSessionStore struct {
	mu                   sync.RWMutex
	byKey                map[string]*relayStreamSession
	activeByConversation map[string]string
}

var relayStreamSessions = &relayStreamSessionStore{
	byKey:                make(map[string]*relayStreamSession),
	activeByConversation: make(map[string]string),
}

func buildRelayStreamSessionKey(conversationID string, requestHash uint64) string {
	return strings.TrimSpace(conversationID) + ":" + strconv.FormatUint(requestHash, 16)
}

func buildRelayConversationScope(conversationID string, apiKeyID int) string {
	return strconv.Itoa(apiKeyID) + ":" + strings.TrimSpace(conversationID)
}

func acquireRelayStreamSession(conversationID string, apiKeyID int, requestHash uint64) (*relayStreamSession, bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || requestHash == 0 {
		return nil, false, nil
	}

	now := time.Now()
	store := relayStreamSessions
	conversationScope := buildRelayConversationScope(conversationID, apiKeyID)
	key := buildRelayStreamSessionKey(conversationScope, requestHash)

	store.mu.Lock()
	defer store.mu.Unlock()

	store.cleanupLocked(now)

	if session, ok := store.byKey[key]; ok {
		return session, false, nil
	}

	if activeKey, ok := store.activeByConversation[conversationScope]; ok && activeKey != key {
		if activeSession, exists := store.byKey[activeKey]; exists && !activeSession.isDoneLocked() {
			return nil, false, errRelayConversationBusy
		}
		delete(store.activeByConversation, conversationScope)
	}

	session := &relayStreamSession{
		store:             store,
		key:               key,
		conversationID:    conversationID,
		conversationScope: conversationScope,
		requestHash:       requestHash,
		createdAt:         now,
		updatedAt:         now,
		subscribers:       make(map[chan struct{}]struct{}),
	}
	store.byKey[key] = session
	store.activeByConversation[conversationScope] = key
	store.enforceSessionLimitLocked()
	return session, true, nil
}

// enforceSessionLimitLocked 在会话总数超过上限时驱逐旧会话，防止全局 map 无界
// 增长。调用方必须持有 store.mu 写锁。
//
// 驱逐分两级：
//  1. 优先驱逐最旧的已完成（done）会话——它们只等清理，驱逐没有副作用。
//  2. 当会话数超过 limit × relayStreamActiveEvictFactor 时，继续按最旧优先驱逐
//     活跃会话，并 Finish 它们释放缓冲。
//
// 第 2 级是这次修复的核心：旧实现「若全是活跃会话则不驱逐」，而客户端中途断连的
// 会话会一直保持活跃直到上游结束，导致硬上限在最需要它的负载下完全失效，map 可以
// 涨到吃光整台机器的内存（issue #196）。丢弃活跃会话确实会打断该会话的重放，但相
// 比整个进程被 OOM killer 杀掉（所有在途请求一起失败）是明显更小的代价。
func (s *relayStreamSessionStore) enforceSessionLimitLocked() {
	limit := getStreamSessionMaxSessions()
	if limit <= 0 || len(s.byKey) <= limit {
		return
	}

	type candidate struct {
		key       string
		scope     string
		session   *relayStreamSession
		updatedAt time.Time
	}
	doneList := make([]candidate, 0, len(s.byKey))
	activeList := make([]candidate, 0, len(s.byKey))
	for key, session := range s.byKey {
		session.mu.RLock()
		done := session.done
		updatedAt := session.updatedAt
		scope := session.conversationScope
		session.mu.RUnlock()
		c := candidate{key: key, scope: scope, session: session, updatedAt: updatedAt}
		if done {
			doneList = append(doneList, c)
		} else {
			activeList = append(activeList, c)
		}
	}
	byOldestFirst := func(list []candidate) func(i, j int) bool {
		return func(i, j int) bool { return list[i].updatedAt.Before(list[j].updatedAt) }
	}
	sort.Slice(doneList, byOldestFirst(doneList))
	sort.Slice(activeList, byOldestFirst(activeList))

	evict := func(c candidate) bool {
		session, ok := s.byKey[c.key]
		if !ok {
			return false
		}
		delete(s.byKey, c.key)
		if activeKey, ok := s.activeByConversation[c.scope]; ok && activeKey == session.key {
			delete(s.activeByConversation, c.scope)
		}
		return true
	}

	excess := len(s.byKey) - limit
	for i := 0; i < len(doneList) && excess > 0; i++ {
		if evict(doneList[i]) {
			excess--
		}
	}

	// 已完成会话清空后仍然超限：说明积压的全是活跃会话。只有在超出宽限阈值
	// （limit × factor）时才动它们，避免正常波动打断正在进行的生成。
	activeLimit := limit
	if relayStreamActiveEvictFactor > 1 {
		activeLimit = limit * relayStreamActiveEvictFactor
	}
	if len(s.byKey) <= activeLimit {
		return
	}
	activeExcess := len(s.byKey) - activeLimit
	evicted := make([]*relayStreamSession, 0, activeExcess)
	for i := 0; i < len(activeList) && activeExcess > 0; i++ {
		if !evict(activeList[i]) {
			continue
		}
		evicted = append(evicted, activeList[i].session)
		activeExcess--
	}
	if len(evicted) == 0 {
		return
	}

	// Finish 会取 store.mu 写锁，而这里正持有它 —— 必须异步调用避免自死锁。
	// 会话已从 map 中摘除，异步 Finish 只影响其订阅者与缓冲释放。
	go func(sessions []*relayStreamSession) {
		for _, session := range sessions {
			session.Finish(errRelayStreamSessionEvicted)
		}
	}(evicted)
	log.Warnf("relay stream session store over hard limit (%d), evicted %d active sessions; "+
		"consider lowering stream_session_max_bytes_mb or raising stream_session_max_sessions",
		activeLimit, len(evicted))
}

// doneSessionRetention 返回已完成会话条目在 map 中的保留时长：取
// relayStreamDoneRetention 与配置 TTL 的较小值。已完成会话的大缓冲在 Finish
// 时已清空，这里控制的是空壳元数据条目的清理时机，与 Finish 调度的窗口一致。
func doneSessionRetention() time.Duration {
	retention := relayStreamDoneRetention
	if ttl := getStreamSessionTTL(); ttl > 0 && ttl < retention {
		retention = ttl
	}
	return retention
}

func (s *relayStreamSessionStore) cleanupLocked(now time.Time) {
	// 保留时长在循环外读取一次，避免在持有 store 写锁期间对每个 session 重复
	// 读取 setting（map 查找 + Atoi）。清理可能遍历大量 session，每次循环都
	// 读 setting 会线性放大写锁的持有时间，阻塞所有新流式会话获取。
	retention := doneSessionRetention()
	for key, session := range s.byKey {
		session.mu.RLock()
		done := session.done
		updatedAt := session.updatedAt
		conversationScope := session.conversationScope
		sessionKey := session.key
		session.mu.RUnlock()

		if !done {
			continue
		}
		if now.Sub(updatedAt) < retention {
			continue
		}

		delete(s.byKey, key)
		if activeKey, ok := s.activeByConversation[conversationScope]; ok && activeKey == sessionKey {
			delete(s.activeByConversation, conversationScope)
		}
	}
}

func (s *relayStreamSessionStore) removeIfExpired(key string, conversationScope string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.byKey[key]
	if !ok {
		return
	}

	session.mu.RLock()
	done := session.done
	updatedAt := session.updatedAt
	sessionKey := session.key
	sessionScope := session.conversationScope
	session.mu.RUnlock()

	if !done || sessionScope != conversationScope || time.Since(updatedAt) < doneSessionRetention() {
		return
	}

	delete(s.byKey, key)
	if activeKey, ok := s.activeByConversation[sessionScope]; ok && activeKey == sessionKey {
		delete(s.activeByConversation, sessionScope)
	}
}

func (s *relayStreamSession) isDoneLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

func (s *relayStreamSession) IsDone() bool {
	if s == nil {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

// MarkClientGone 记录「客户端已断连、但上游仍在生成」的时刻，开始宽限计时。
// 重复调用只有第一次生效，保证宽限期从第一次断连算起。
func (s *relayStreamSession) MarkClientGone() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clientGoneActive || s.done {
		return
	}
	s.clientGoneActive = true
	s.clientGoneAt = time.Now()
}

// HasSubscribers 报告当前是否有读取者订阅该会话（即客户端是否已重连回来读）。
func (s *relayStreamSession) HasSubscribers() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers) > 0
}

// ClientGoneGraceExceeded 报告客户端断连后的宽限期是否已耗尽。
// 调用方据此强制结束会话并停止读取上游，把断连会话的缓冲驻留时间限定在宽限期内。
func (s *relayStreamSession) ClientGoneGraceExceeded() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.clientGoneActive || s.done {
		return false
	}
	return time.Since(s.clientGoneAt) >= relayStreamClientGoneGrace
}

func (s *relayStreamSession) AddPayload(payload []byte) []relayStreamEvent {
	if s == nil || len(payload) == 0 {
		return nil
	}

	frames := splitRelaySSEPayload(payload)
	if len(frames) == 0 {
		return nil
	}

	events := make([]relayStreamEvent, 0, len(frames))

	s.mu.Lock()
	for _, frame := range frames {
		s.nextSeq++
		event := relayStreamEvent{
			Sequence: s.nextSeq,
			Payload:  frame,
		}
		s.events = append(s.events, event)
		s.bufferBytes += len(frame)
		events = append(events, event)
	}
	s.trimEventsLocked()
	s.updatedAt = time.Now()

	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	return events
}

func (s *relayStreamSession) trimEventsLocked() {
	// maxEvents / maxBytes 在循环外读取一次：trimEventsLocked 由每个流式帧的
	// AddPayload 调用，循环内重复读 setting 会在热路径上放大开销。
	maxEvents := getStreamSessionMaxEvents()
	maxBytes := getStreamSessionMaxBytes()
	for len(s.events) > 0 {
		tooManyEvents := maxEvents > 0 && len(s.events) > maxEvents
		tooManyBytes := maxBytes > 0 && s.bufferBytes > maxBytes && len(s.events) > 1
		if !tooManyEvents && !tooManyBytes {
			return
		}

		dropped := s.events[0]
		s.droppedBeforeSeq = dropped.Sequence
		s.bufferBytes -= len(dropped.Payload)
		if s.bufferBytes < 0 {
			s.bufferBytes = 0
		}
		s.events[0].Payload = nil
		s.events = s.events[1:]
	}
}

func (s *relayStreamSession) Snapshot(afterSeq int64) ([]relayStreamEvent, bool, error) {
	if s == nil {
		return nil, true, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if afterSeq < s.droppedBeforeSeq {
		return nil, s.done, errRelayReplayWindowExpired
	}

	idx := 0
	for idx < len(s.events) && s.events[idx].Sequence <= afterSeq {
		idx++
	}

	events := make([]relayStreamEvent, 0, len(s.events)-idx)
	for ; idx < len(s.events); idx++ {
		event := s.events[idx]
		event.Payload = append([]byte(nil), event.Payload...)
		events = append(events, event)
	}

	return events, s.done, s.err
}

func (s *relayStreamSession) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	if s == nil {
		close(ch)
		return ch, func() {}
	}

	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	s.subscribers[ch] = struct{}{}
	// 有订阅者接入意味着客户端已经重连回来读这个会话，撤销断连宽限计时，
	// 否则原转发协程仍会在宽限期到点时把这个「有人在读」的会话强行结束
	// （原请求的 context 早已 Done，无法自行感知重连）。
	s.clientGoneActive = false
	s.clientGoneAt = time.Time{}
	s.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			s.mu.Lock()
			if _, ok := s.subscribers[ch]; ok {
				delete(s.subscribers, ch)
				close(ch)
			}
			s.mu.Unlock()
		})
	}

	return ch, unsubscribe
}

func (s *relayStreamSession) Finish(err error) {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return
	}
	s.done = true
	s.err = err
	s.updatedAt = time.Now()

	// 根据设置决定是否清空 events：关闭重连重放功能可立即释放大缓冲，
	// 降低内存占用（见 OOM 诊断报告）。开启时保留缓冲支持断线重连重放
	// （客户端重连后先重放已缓冲事件，再收到 terminal error）。
	replayEnabled, err := setting.GetBool(dbmodel.SettingKeyStreamSessionReplayEnabled)
	if err == nil && !replayEnabled {
		s.events = nil // 立即释放缓冲区
	}
	// 若启用重连重放（或读取设置失败，保守保留），内存控制改由
	// 缩短的保留窗口实现——见 relayStreamDoneRetention：done 会话最多保留 2 分钟
	// （而非完整 TTL 30 分钟）即被清理，把大缓冲的驻留时间压缩一个数量级
	// （见 issue #46 的内存暴涨）。

	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subscribers = append(subscribers, ch)
	}
	s.subscribers = make(map[chan struct{}]struct{})
	s.mu.Unlock()

	s.store.mu.Lock()
	if activeKey, ok := s.store.activeByConversation[s.conversationScope]; ok && activeKey == s.key {
		delete(s.store.activeByConversation, s.conversationScope)
	}
	s.store.mu.Unlock()

	// 用较短的完成保留窗口调度清理（取 doneRetention 与 TTL 的较小值），
	// 而非完整 TTL，缩短已完成会话条目在 map 中的驻留时间。
	retention := relayStreamDoneRetention
	if ttl := getStreamSessionTTL(); ttl > 0 && ttl < retention {
		retention = ttl
	}
	if retention > 0 {
		time.AfterFunc(retention, func() {
			s.store.removeIfExpired(s.key, s.conversationScope)
		})
	}

	for _, ch := range subscribers {
		close(ch)
	}
}

func splitRelaySSEPayload(payload []byte) [][]byte {
	trimmed := bytes.TrimLeft(payload, "\r\n")
	if len(trimmed) == 0 {
		return nil
	}

	parts := bytes.Split(trimmed, []byte("\n\n"))
	frames := make([][]byte, 0, len(parts))
	for _, part := range parts {
		frame := bytes.TrimLeft(part, "\r\n")
		if len(bytes.TrimSpace(frame)) == 0 {
			continue
		}
		cloned := append([]byte(nil), frame...)
		if !bytes.HasSuffix(cloned, []byte("\n\n")) {
			cloned = append(cloned, '\n', '\n')
		}
		frames = append(frames, cloned)
	}
	return frames
}

func formatRelaySSEEvent(sequence int64, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+32)
	frame = append(frame, []byte("id: "+strconv.FormatInt(sequence, 10)+"\n")...)
	frame = append(frame, payload...)
	if !bytes.HasSuffix(frame, []byte("\n\n")) {
		frame = append(frame, '\n', '\n')
	}
	return frame
}

func writeSSEErrorEvent(w io.Writer, message string) {
	data, _ := jsonAPI.Marshal(map[string]string{"error": message})
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
}

func serveRelayStreamSession(c *gin.Context, req *relayRequest) {
	if req == nil || req.streamSession == nil {
		resp.Error(c, http.StatusBadRequest, "missing relay stream session")
		return
	}

	clientCtx := c.Request.Context()
	lastSeq := req.internalRequest.ResumeFromEventID
	headersWritten := false

	writeHeaders := func() {
		if headersWritten {
			return
		}
		headersWritten = true
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Header("X-Conversation-ID", req.internalRequest.ConversationID)
	}

	writeEvents := func(events []relayStreamEvent) bool {
		for _, event := range events {
			writeHeaders()
			if _, err := c.Writer.Write(formatRelaySSEEvent(event.Sequence, event.Payload)); err != nil {
				return false
			}
			c.Writer.Flush()
			lastSeq = event.Sequence
		}
		return true
	}

	sub, unsubscribe := req.streamSession.Subscribe()
	defer unsubscribe()

	heartbeatTicker := time.NewTicker(conf.SSEHeartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		events, done, sessionErr := req.streamSession.Snapshot(lastSeq)
		if errors.Is(sessionErr, errRelayReplayWindowExpired) {
			if !headersWritten {
				resp.Error(c, http.StatusConflict, sessionErr.Error())
			} else {
				writeHeaders()
				writeSSEErrorEvent(c.Writer, sessionErr.Error())
				c.Writer.Flush()
			}
			return
		}
		if len(events) > 0 {
			if !writeEvents(events) {
				return
			}
		}

		if done {
			if sessionErr != nil {
				if !headersWritten {
					statusCode := http.StatusBadGateway
					if errors.Is(sessionErr, context.DeadlineExceeded) {
						statusCode = http.StatusGatewayTimeout
					}
					resp.Error(c, statusCode, sessionErr.Error())
				} else {
					writeHeaders()
					writeSSEErrorEvent(c.Writer, sessionErr.Error())
					c.Writer.Flush()
				}
			}
			return
		}

		select {
		case <-clientCtx.Done():
			return
		case <-heartbeatTicker.C:
			if headersWritten {
				if _, err := c.Writer.Write([]byte(": ping\n\n")); err != nil {
					return
				}
				c.Writer.Flush()
			}
		case _, ok := <-sub:
			if !ok {
				continue
			}
		}
	}
}

// ActiveSessionCount returns the count of active (not yet done) stream sessions.
func ActiveSessionCount() int {
	relayStreamSessions.mu.RLock()
	defer relayStreamSessions.mu.RUnlock()
	count := 0
	for _, s := range relayStreamSessions.byKey {
		if !s.IsDone() {
			count++
		}
	}
	return count
}

// PurgeExpiredStreamSessions proactively removes finished stream sessions whose
// retention window has elapsed. It is invoked by a periodic background task so
// cleanup does not depend solely on a new session being acquired (lazy
// cleanupLocked) or on per-session AfterFunc timers. This bounds the global
// session map under sustained streaming load (see issue #46).
//
// 它同时执行会话总数硬上限（enforceSessionLimitLocked）。之前硬上限只在获取新会话
// 时检查，而 OOM 场景恰恰是会话只增不减、旧会话又都处于活跃状态——把检查放到周期
// 任务里，保证超限积压总能被收敛，不依赖新请求到达。
func PurgeExpiredStreamSessions() {
	store := relayStreamSessions
	store.mu.Lock()
	defer store.mu.Unlock()
	store.cleanupLocked(time.Now())
	store.enforceSessionLimitLocked()
}
