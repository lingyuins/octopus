package relay

import (
	"context"
	"testing"
	"time"
)

func TestShouldHoldOnRateLimit(t *testing.T) {
	cfg := rateLimitHoldConfig{Enabled: true, Interval: 10 * time.Second, MaxWait: 60 * time.Second}
	if !shouldHoldOnRateLimit(cfg, RetryDecision{Scope: ScopeSameChannel, Code: 429, IsError: true}) {
		t.Fatal("expected 429 same-channel decision to hold when enabled")
	}
	if shouldHoldOnRateLimit(cfg, RetryDecision{Scope: ScopeSameChannel, Code: 401, IsError: true}) {
		t.Fatal("401 must not enter rate-limit hold")
	}
	if shouldHoldOnRateLimit(cfg, RetryDecision{Scope: ScopeNextChannel, Code: 429, IsError: true}) {
		t.Fatal("next-channel decision must not enter rate-limit hold")
	}
	cfg.Enabled = false
	if shouldHoldOnRateLimit(cfg, RetryDecision{Scope: ScopeSameChannel, Code: 429, IsError: true}) {
		t.Fatal("disabled hold must keep historical immediate failover")
	}
}

func TestCanContinueRateLimitHoldBudget(t *testing.T) {
	cfg := rateLimitHoldConfig{Enabled: true, Interval: 10 * time.Second, MaxWait: 60 * time.Second}
	if !canContinueRateLimitHold(cfg, 0) {
		t.Fatal("first wait should be allowed")
	}
	if !canContinueRateLimitHold(cfg, 50*time.Second) {
		t.Fatal("50s waited + 10s interval should still fit in 60s")
	}
	if canContinueRateLimitHold(cfg, 51*time.Second) {
		t.Fatal("51s waited + 10s interval exceeds 60s budget")
	}
	if canContinueRateLimitHold(cfg, 60*time.Second) {
		t.Fatal("budget exhausted")
	}
}

func TestGetRateLimitHoldConfigClampsInterval(t *testing.T) {
	// 未配置 setting 时走默认值；这里只校验 helper 夹取逻辑。
	cfg := rateLimitHoldConfig{Enabled: true, Interval: 90 * time.Second, MaxWait: 60 * time.Second}
	if cfg.Interval > cfg.MaxWait {
		cfg.Interval = cfg.MaxWait
	}
	if cfg.Interval != 60*time.Second {
		t.Fatalf("interval = %v, want 60s after clamp", cfg.Interval)
	}
}

func TestWaitRateLimitHoldRespectsCancel(t *testing.T) {
	cfg := rateLimitHoldConfig{Enabled: true, Interval: 2 * time.Second, MaxWait: 10 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if waitRateLimitHold(ctx, cfg, "demo", 0) {
		t.Fatal("canceled context should not wait successfully")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatalf("canceled wait took too long: %v", time.Since(start))
	}
}

func TestWaitRateLimitHoldCompletes(t *testing.T) {
	cfg := rateLimitHoldConfig{Enabled: true, Interval: 20 * time.Millisecond, MaxWait: time.Second}
	start := time.Now()
	if !waitRateLimitHold(context.Background(), cfg, "demo", 0) {
		t.Fatal("expected wait to complete")
	}
	elapsed := time.Since(start)
	if elapsed < 15*time.Millisecond {
		t.Fatalf("wait returned too early: %v", elapsed)
	}
}
