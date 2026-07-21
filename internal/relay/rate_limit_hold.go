package relay

import (
	"context"
	"time"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"github.com/lingyuins/octopus/internal/utils/log"
)

const (
	defaultRateLimitHoldIntervalSec = 10
	defaultRateLimitHoldMaxWaitSec  = 60
)

// rateLimitHoldConfig 描述「渠道内 429 延时重试」策略。
// 默认关闭，行为与历史一致：429 立刻换 Key / 渠道。
type rateLimitHoldConfig struct {
	Enabled  bool
	Interval time.Duration
	MaxWait  time.Duration
}

func getRateLimitHoldConfig() rateLimitHoldConfig {
	cfg := rateLimitHoldConfig{
		Enabled:  false,
		Interval: time.Duration(defaultRateLimitHoldIntervalSec) * time.Second,
		MaxWait:  time.Duration(defaultRateLimitHoldMaxWaitSec) * time.Second,
	}

	if v, err := setting.GetBool(dbmodel.SettingKeyRateLimitHoldEnabled); err == nil {
		cfg.Enabled = v
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyRateLimitHoldInterval); err == nil && v > 0 {
		cfg.Interval = time.Duration(v) * time.Second
	}
	if v, err := setting.GetInt(dbmodel.SettingKeyRateLimitHoldMaxWait); err == nil && v > 0 {
		cfg.MaxWait = time.Duration(v) * time.Second
	}
	// 间隔不应超过总等待上限，否则一次等待就会直接耗尽预算。
	if cfg.Interval > cfg.MaxWait {
		cfg.Interval = cfg.MaxWait
	}
	return cfg
}

// shouldHoldOnRateLimit 判断本次失败是否应进入「当前渠道内延时重试」。
// 仅对真正的 429 生效；其它 ScopeSameChannel（401/403/空输出）保持原立即换 Key。
func shouldHoldOnRateLimit(cfg rateLimitHoldConfig, decision RetryDecision) bool {
	return cfg.Enabled && decision.Scope == ScopeSameChannel && decision.Code == 429
}

// canContinueRateLimitHold 在累计等待后是否还能再等一轮 interval。
// 允许 elapsed==0 时至少尝试一轮等待；当剩余预算不足一整轮 interval 时停止 hold。
func canContinueRateLimitHold(cfg rateLimitHoldConfig, waited time.Duration) bool {
	if !cfg.Enabled || cfg.Interval <= 0 || cfg.MaxWait <= 0 {
		return false
	}
	return waited+cfg.Interval <= cfg.MaxWait
}

// waitRateLimitHold 阻塞等待下一轮 429 重试，同时响应客户端/操作上下文取消。
// 返回 true 表示等待完成可继续重试；false 表示上下文已取消。
func waitRateLimitHold(ctx context.Context, cfg rateLimitHoldConfig, channelName string, waited time.Duration) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	remainingBudget := cfg.MaxWait - waited
	waitFor := cfg.Interval
	if remainingBudget > 0 && remainingBudget < waitFor {
		waitFor = remainingBudget
	}
	if waitFor <= 0 {
		return false
	}

	log.Infof("rate limit hold: channel=%s wait=%s elapsed=%s max=%s",
		channelName, waitFor, waited.Round(time.Millisecond), cfg.MaxWait)

	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
