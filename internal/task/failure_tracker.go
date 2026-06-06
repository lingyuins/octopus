package task

import (
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/utils/log"
)

const (
	// maxConsecutiveFailures 连续失败次数阈值，超过后进入冷却期
	maxConsecutiveFailures = 3

	// cooldownDuration 冷却期时长，期间跳过该 channel
	cooldownDuration = 30 * time.Minute
)

type channelFailureState struct {
	consecutiveFailures int
	cooldownUntil       time.Time
}

type FailureTracker struct {
	mu     sync.Mutex
	states map[int]*channelFailureState // keyed by channel ID
}

// NewFailureTracker 创建新的失败追踪器
func NewFailureTracker() *FailureTracker {
	return &FailureTracker{
		states: make(map[int]*channelFailureState),
	}
}

// ShouldSkip 检查 channel 是否应该被跳过（在冷却期内）
func (ft *FailureTracker) ShouldSkip(channelID int) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	state, ok := ft.states[channelID]
	if !ok {
		return false
	}

	if !state.cooldownUntil.IsZero() && time.Now().Before(state.cooldownUntil) {
		return true
	}

	return false
}

// RecordFailure 记录失败，达到阈值时进入冷却期
func (ft *FailureTracker) RecordFailure(channelID int, channelName string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	state, ok := ft.states[channelID]
	if !ok {
		state = &channelFailureState{}
		ft.states[channelID] = state
	}

	state.consecutiveFailures++

	if state.consecutiveFailures >= maxConsecutiveFailures {
		state.cooldownUntil = time.Now().Add(cooldownDuration)
		log.Warnf("channel %q (id=%d) entered cooldown after %d consecutive failures, "+
			"will retry after %s",
			channelName, channelID, state.consecutiveFailures,
			state.cooldownUntil.Format(time.RFC3339))
	}
}

// RecordSuccess 记录成功，重置失败计数
func (ft *FailureTracker) RecordSuccess(channelID int) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	state, ok := ft.states[channelID]
	if !ok {
		return
	}

	if state.consecutiveFailures > 0 {
		log.Infof("channel id=%d recovered after %d failures", channelID, state.consecutiveFailures)
	}

	state.consecutiveFailures = 0
	state.cooldownUntil = time.Time{}
}

// Cleanup 清理过期条目，防止 map 无限增长
func (ft *FailureTracker) Cleanup() {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	for id, state := range ft.states {
		// 成功且冷却期已过
		if state.consecutiveFailures == 0 && state.cooldownUntil.IsZero() {
			delete(ft.states, id)
		}
		// 冷却期已过且没有新失败
		if !state.cooldownUntil.IsZero() && time.Now().After(state.cooldownUntil.Add(cooldownDuration)) {
			delete(ft.states, id)
		}
	}
}
