package relay

import (
	"runtime"
	"sync/atomic"
	"time"

	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/relay/balancer"
	"github.com/lingyuins/octopus/internal/utils/telemetry"
)

var processStartedAt = time.Now()

var inflightRequests int64

const trendPoints = 12
const trendInterval = 30 * time.Second

type RuntimeTrendSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestDelta int64     `json:"request_delta"`
	FailedDelta  int64     `json:"failed_delta"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	MemoryMB     int64     `json:"memory_mb"`
}

var (
	trendSnapshots   [trendPoints]RuntimeTrendSnapshot
	trendSnapshotIdx int
	trendTotalReqs   int64
	trendFailedReqs  int64
	trendTotalLatMs  int64
)

func init() {
	go trendWorker()
	go sessionMetricsWorker()
}

func trendWorker() {
	ticker := time.NewTicker(trendInterval)
	defer ticker.Stop()
	for range ticker.C {
		sampleTrendSnapshot()
	}
}

func sampleTrendSnapshot() {
	deltaReqs := atomic.SwapInt64(&trendTotalReqs, 0)
	deltaFailed := atomic.SwapInt64(&trendFailedReqs, 0)
	deltaLatMs := atomic.SwapInt64(&trendTotalLatMs, 0)

	var avgLat float64
	if deltaReqs > 0 {
		avgLat = float64(deltaLatMs) / float64(deltaReqs)
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	idx := trendSnapshotIdx % trendPoints
	trendSnapshots[idx] = RuntimeTrendSnapshot{
		Timestamp:    time.Now(),
		RequestDelta: deltaReqs,
		FailedDelta:  deltaFailed,
		AvgLatencyMs: avgLat,
		MemoryMB:     int64(mem.Alloc / (1024 * 1024)),
	}
	trendSnapshotIdx++
}

func InflightCount() int64 {
	return atomic.LoadInt64(&inflightRequests)
}

func InflightInc() int64 {
	telemetry.Global().ActiveConnectionsInc()
	return atomic.AddInt64(&inflightRequests, 1)
}

func InflightDec() int64 {
	telemetry.Global().ActiveConnectionsDec()
	return atomic.AddInt64(&inflightRequests, -1)
}

func UptimeSeconds() int64 {
	return int64(time.Since(processStartedAt).Seconds())
}

func ProcessMemoryMB() int64 {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return int64(mem.Alloc / (1024 * 1024))
}

func TrendSnapshots() []RuntimeTrendSnapshot {
	count := trendSnapshotIdx
	if count > trendPoints {
		count = trendPoints
	}
	result := make([]RuntimeTrendSnapshot, 0, count)
	start := trendSnapshotIdx - count
	if start < 0 {
		start = 0
	}
	for i := start; i < trendSnapshotIdx; i++ {
		idx := i % trendPoints
		snap := trendSnapshots[idx]
		if !snap.Timestamp.IsZero() {
			result = append(result, snap)
		}
	}
	return result
}

func RecordRequest(latencyMs int64, failed bool) {
	atomic.AddInt64(&trendTotalReqs, 1)
	atomic.AddInt64(&trendTotalLatMs, latencyMs)
	if failed {
		atomic.AddInt64(&trendFailedReqs, 1)
	}
}

// sessionMetricsWorker periodically pushes session/sticky counts to the shared telemetry store
// so that ops can read them without an import cycle.
func sessionMetricsWorker() {
	ticker := time.NewTicker(conf.SSEHeartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		telemetry.Global().SetActiveSessions(int64(ActiveSessionCount()))
		telemetry.Global().SetStickyBoundSessions(int64(balancer.StickyCount()))
	}
}
