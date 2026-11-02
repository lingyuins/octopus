package nodetransport

import (
	"testing"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
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
