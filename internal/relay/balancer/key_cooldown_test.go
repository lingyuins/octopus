package balancer

import (
	"net/http"
	"testing"
	"time"
)

// resetKeyCooldown 清空 globalKeyCooldown，保证测试隔离。
func resetKeyCooldown() {
	globalKeyCooldown.Range(func(k, _ any) bool { globalKeyCooldown.Delete(k); return true })
}

func TestIsKeyOnCooldownFalseWhenNoEntry(t *testing.T) {
	resetKeyCooldown()
	if IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("expected no cooldown when no entry recorded")
	}
}

func TestRecordKeyCooldownBlocksSameModel(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	if !IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("same (channel,key,model) should be on cooldown after recording")
	}
}

func TestRecordKeyCooldownDoesNotBlockOtherModel(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	// 同一 key 对另一个模型不应被冷却——这是 issue #94 的核心诉求。
	if IsKeyOnCooldown(1, 1, "claude-3-5-sonnet") {
		t.Fatal("other model on same key should not be cooled down")
	}
}

func TestRecordKeyCooldownDoesNotBlockOtherKey(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	if IsKeyOnCooldown(1, 2, "gpt-4o") {
		t.Fatal("other key on same model should not be cooled down")
	}
}

func TestIsKeyOnCooldownFalseAfterExpiry(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)

	// 回退到期时间模拟过期
	key := cooldownKey(1, 1, "gpt-4o")
	v, ok := globalKeyCooldown.Load(key)
	if !ok {
		t.Fatal("cooldown entry missing after record")
	}
	entry := v.(*keyCooldownEntry)
	entry.expiresAt = time.Now().Add(-time.Second)

	if IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("expired cooldown should report not on cooldown")
	}
	if _, ok := globalKeyCooldown.Load(key); ok {
		t.Fatal("expired cooldown entry should be lazily purged")
	}
}

func TestIsKeyOnCooldownIgnoresEmptyModel(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusTooManyRequests)
	// 空 model（后台任务场景）不应被冷却拦截。
	if IsKeyOnCooldown(1, 1, "") {
		t.Fatal("empty model should bypass cooldown")
	}
}

func TestRecordKeyCooldownIgnoredForSuccessStatus(t *testing.T) {
	resetKeyCooldown()
	// 2xx 不应触发冷却。
	RecordKeyCooldown(1, 1, "gpt-4o", http.StatusOK)
	if IsKeyOnCooldown(1, 1, "gpt-4o") {
		t.Fatal("2xx status should not record cooldown")
	}
}

func TestPurgeExpiredKeyCooldownsRemovesExpired(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "fresh", http.StatusTooManyRequests)
	RecordKeyCooldown(1, 1, "stale", http.StatusTooManyRequests)

	// 回退 stale 条目到期时间
	key := cooldownKey(1, 1, "stale")
	if v, ok := globalKeyCooldown.Load(key); ok {
		v.(*keyCooldownEntry).expiresAt = time.Now().Add(-time.Second)
	}

	removed := PurgeExpiredKeyCooldowns()
	if removed < 1 {
		t.Fatalf("PurgeExpiredKeyCooldowns removed %d, want >= 1", removed)
	}
	if _, ok := globalKeyCooldown.Load(key); ok {
		t.Fatal("expired cooldown entry should have been purged")
	}
	if _, ok := globalKeyCooldown.Load(cooldownKey(1, 1, "fresh")); !ok {
		t.Fatal("fresh cooldown entry should remain")
	}
}

func TestRemoveChannelKeyCooldownsScopedByChannel(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 1, "model-a", http.StatusTooManyRequests)
	RecordKeyCooldown(1, 2, "model-b", http.StatusTooManyRequests)
	RecordKeyCooldown(2, 3, "model-c", http.StatusTooManyRequests)

	RemoveChannelKeyCooldowns(1)

	if IsKeyOnCooldown(1, 1, "model-a") {
		t.Fatal("channel 1 model-a should have been removed")
	}
	if IsKeyOnCooldown(1, 2, "model-b") {
		t.Fatal("channel 1 model-b should have been removed")
	}
	// 其他渠道的条目应保留
	if !IsKeyOnCooldown(2, 3, "model-c") {
		t.Fatal("channel 2 model-c should remain")
	}
}

func TestClearKeyCooldownAllowsReuse(t *testing.T) {
	resetKeyCooldown()
	RecordKeyCooldown(1, 2, "gpt-4o", http.StatusTooManyRequests)
	if !IsKeyOnCooldown(1, 2, "gpt-4o") {
		t.Fatal("expected cooldown after record")
	}
	ClearKeyCooldown(1, 2, "gpt-4o")
	if IsKeyOnCooldown(1, 2, "gpt-4o") {
		t.Fatal("expected cooldown cleared")
	}
}
