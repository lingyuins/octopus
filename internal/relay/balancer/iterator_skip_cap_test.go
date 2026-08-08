package balancer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/model"
)

func newTestIterator(t *testing.T) *Iterator {
	t.Helper()
	group := model.Group{
		Items: []model.GroupItem{{ChannelID: 1, ModelName: "gpt-4o"}},
	}
	it := NewIterator(group, 0, "gpt-4o", nil)
	if !it.Next() {
		t.Fatal("iterator has no candidates")
	}
	return it
}

// 跳过明细超过 maxSkipAttemptRecords 后不再膨胀，Attempts 以一条汇总记录
// 说明省略数量（issue #192：无上限时单条 relay log 曾膨胀到数百 MB）。
func TestIterator_SkipRecordsCapped(t *testing.T) {
	it := newTestIterator(t)

	const total = maxSkipAttemptRecords + 500
	for i := 0; i < total; i++ {
		it.Skip(1, 1, "ch", "no available key")
	}

	attempts := it.Attempts()
	if len(attempts) != maxSkipAttemptRecords+1 {
		t.Fatalf("len(attempts) = %d, want %d (cap + 1 条汇总)", len(attempts), maxSkipAttemptRecords+1)
	}
	last := attempts[len(attempts)-1]
	if !strings.Contains(last.Msg, fmt.Sprintf("%d", total-maxSkipAttemptRecords)) {
		t.Fatalf("汇总记录未包含省略条数: %q", last.Msg)
	}
	if last.AttemptNum != total {
		t.Fatalf("汇总记录 AttemptNum = %d, want %d（保留总计数）", last.AttemptNum, total)
	}
}

// 未超限时 Attempts 不追加汇总记录，行为与旧版一致。
func TestIterator_SkipRecordsUnderCapUnchanged(t *testing.T) {
	it := newTestIterator(t)
	for i := 0; i < 10; i++ {
		it.Skip(1, 1, "ch", "reason")
	}
	if got := len(it.Attempts()); got != 10 {
		t.Fatalf("len(attempts) = %d, want 10", got)
	}
}

// 跳过截断不影响真实转发记录与 ForwardedAttempts 语义
// （最大总尝试次数配额只数真实转发）。
func TestIterator_ForwardedAttemptsUnaffectedByCap(t *testing.T) {
	it := newTestIterator(t)

	for i := 0; i < maxSkipAttemptRecords+50; i++ {
		it.Skip(1, 1, "ch", "skip")
	}
	for i := 0; i < 3; i++ {
		span := it.StartAttempt(1, 1, "ch", "gpt-4o")
		span.End(model.AttemptFailed, 500, "upstream error")
	}

	if got := it.ForwardedAttempts(); got != 3 {
		t.Fatalf("ForwardedAttempts() = %d, want 3", got)
	}
	// 真实转发记录必须保留在明细里（跳过截断只丢跳过记录）。
	forwarded := 0
	for _, a := range it.Attempts() {
		if a.Status == model.AttemptFailed {
			forwarded++
		}
	}
	if forwarded != 3 {
		t.Fatalf("明细中的真实转发记录 = %d, want 3", forwarded)
	}
}

// SkipCircuitBreak 与 Skip 共享同一上限。
func TestIterator_CircuitBreakRecordsShareCap(t *testing.T) {
	it := newTestIterator(t)

	// 连续失败超过默认阈值（5）触发熔断。
	for i := 0; i < 6; i++ {
		RecordFailure(9101, 9102, "gpt-4o")
	}
	defer RecordSuccess(9101, 9102, "gpt-4o")

	for i := 0; i < maxSkipAttemptRecords+20; i++ {
		if !it.SkipCircuitBreak(9101, 9102, "ch", "gpt-4o") {
			t.Fatal("SkipCircuitBreak 应返回 true（已熔断）")
		}
	}
	if got := len(it.Attempts()); got != maxSkipAttemptRecords+1 {
		t.Fatalf("len(attempts) = %d, want %d", got, maxSkipAttemptRecords+1)
	}
}
