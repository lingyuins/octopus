package nodetransport

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

func setNodeTransportConfig(t *testing.T, frames int, wait time.Duration) {
	t.Helper()
	prevFrames := conf.AppConfig.NodeTransport.StreamQueueFrames
	prevWait := conf.AppConfig.NodeTransport.ConsumerWaitTimeout
	conf.AppConfig.NodeTransport.StreamQueueFrames = frames
	conf.AppConfig.NodeTransport.ConsumerWaitTimeout = wait
	t.Cleanup(func() {
		conf.AppConfig.NodeTransport.StreamQueueFrames = prevFrames
		conf.AppConfig.NodeTransport.ConsumerWaitTimeout = prevWait
	})
}

// setNodeTransportDBSetting 直接写 setting 缓存（DB setting 生效路径），
// 并在 Cleanup 恢复旧值，避免泄漏到其他测试。
func setNodeTransportDBSetting(t *testing.T, key model.SettingKey, value string) {
	t.Helper()
	c := setting.GetCache()
	old := c.GetAll()
	c.Set(key, value)
	t.Cleanup(func() {
		c.Clear()
		for k, v := range old {
			c.Set(k, v)
		}
	})
}

func TestStreamQueueFramesConfig(t *testing.T) {
	setNodeTransportConfig(t, 0, 0)
	if got := streamQueueFrames(); got != defaultStreamQueue {
		t.Fatalf("zero frames should fall back to default %d, got %d", defaultStreamQueue, got)
	}
	setNodeTransportConfig(t, 64, 0)
	if got := streamQueueFrames(); got != 64 {
		t.Fatalf("configured 64, got %d", got)
	}
	setNodeTransportConfig(t, -5, 0)
	if got := streamQueueFrames(); got != defaultStreamQueue {
		t.Fatalf("negative frames should fall back to default %d, got %d", defaultStreamQueue, got)
	}
}

func TestConsumerWaitTimeoutConfig(t *testing.T) {
	setNodeTransportConfig(t, 0, 0)
	if got := consumerWaitTimeout(); got != 0 {
		t.Fatalf("zero wait should stay 0 (legacy immediate fail), got %v", got)
	}
	setNodeTransportConfig(t, 0, 10*time.Second)
	if got := consumerWaitTimeout(); got != 10*time.Second {
		t.Fatalf("configured 10s, got %v", got)
	}
}

func TestDBSettingTakesPrecedenceOverConfig(t *testing.T) {
	setNodeTransportConfig(t, 16, 3*time.Second)
	setNodeTransportDBSetting(t, model.SettingKeyNodeTransportStreamQueueFrames, "128")
	setNodeTransportDBSetting(t, model.SettingKeyNodeTransportConsumerWaitTimeout, "10")
	if got := streamQueueFrames(); got != 128 {
		t.Fatalf("DB setting should win over config: got %d, want 128", got)
	}
	if got := consumerWaitTimeout(); got != 10*time.Second {
		t.Fatalf("DB setting should win over config: got %v, want 10s", got)
	}
}
