// Package telemetry provides a lightweight, import-cycle-free runtime metrics store.
// Both internal/relay (writer) and internal/op/ops (reader) use this package without
// creating a dependency cycle.
package telemetry

import (
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxLatencySamples   = 10000 // rolling buffer for P95 calculation
	maxTrendSnapshots   = 60    // 1 snapshot per minute = 1 hour of history
	trendSnapshotPeriod = 60 * time.Second
)

// Store holds all runtime metrics in a concurrency-safe way.
type Store struct {
	mu sync.Mutex

	// Counter metrics (atomic for high-frequency writes)
	totalRequests atomic.Int64
	totalFailures atomic.Int64
	totalWaitMs   atomic.Int64
	activeConns   atomic.Int64
	throughputRPS atomic.Int64 // updated once per second by background goroutine

	// Session metrics (updated periodically by relay)
	activeSessions      atomic.Int64
	stickyBoundSessions atomic.Int64
	quotaAlerts         atomic.Int64

	// Latency sampling (mutex-protected ring buffer)
	latencySamples []float64
	latencyCursor  int
	latencyFull    bool

	// Trend snapshots (mutex-protected)
	trendSnapshots []TrendPoint
	lastSnapshotAt time.Time

	// Background ticker control
	started  bool
	stopChan chan struct{}
}

// TrendPoint is a single time-series data point.
type TrendPoint struct {
	Timestamp    int64   `json:"timestamp"`
	RequestDelta int64   `json:"request_delta"`
	FailedDelta  int64   `json:"failed_delta"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	MemoryMB     int64   `json:"memory_mb"`
}

var globalStore = NewStore()

// Global returns the global metrics store.
func Global() *Store { return globalStore }

// NewStore creates a new metrics store.
func NewStore() *Store {
	return &Store{
		latencySamples: make([]float64, 0, maxLatencySamples),
	}
}

// StartBackground begins periodic metric collection (throughput, trend snapshots).
// Safe to call multiple times.
func (s *Store) StartBackground() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	s.stopChan = make(chan struct{})

	// Throughput ticker: every second, snapshot the request counter into throughputRPS
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		var lastReqCount int64
		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				current := s.totalRequests.Load()
				delta := current - lastReqCount
				lastReqCount = current
				s.throughputRPS.Store(delta)
			}
		}
	}()

	// Trend snapshot ticker: every minute, capture a data point
	go func() {
		ticker := time.NewTicker(trendSnapshotPeriod)
		defer ticker.Stop()
		var lastReqCount int64
		var lastFailCount int64
		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				currentReq := s.totalRequests.Load()
				currentFail := s.totalFailures.Load()

				reqDelta := currentReq - lastReqCount
				failDelta := currentFail - lastFailCount
				lastReqCount = currentReq
				lastFailCount = currentFail

				var avgLatency float64
				if s.totalRequests.Load() > 0 {
					avgLatency = float64(s.totalWaitMs.Load()) / float64(s.totalRequests.Load())
				}

				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				memMB := int64(memStats.Alloc / 1024 / 1024)

				point := TrendPoint{
					Timestamp:    time.Now().Unix(),
					RequestDelta: reqDelta,
					FailedDelta:  failDelta,
					AvgLatencyMs: avgLatency,
					MemoryMB:     memMB,
				}

				s.mu.Lock()
				s.trendSnapshots = append(s.trendSnapshots, point)
				if len(s.trendSnapshots) > maxTrendSnapshots {
					s.trendSnapshots = s.trendSnapshots[1:]
				}
				s.mu.Unlock()
			}
		}
	}()
}

// StopBackground stops the background collection goroutines.
func (s *Store) StopBackground() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopChan != nil {
		close(s.stopChan)
		s.stopChan = nil
	}
	s.started = false
}

// RecordRequest records a completed relay request.
func (s *Store) RecordRequest(latencyMs int64, success bool) {
	s.totalRequests.Add(1)
	s.totalWaitMs.Add(latencyMs)
	if !success {
		s.totalFailures.Add(1)
	}

	s.mu.Lock()
	if s.latencyFull {
		s.latencySamples[s.latencyCursor] = float64(latencyMs)
	} else {
		s.latencySamples = append(s.latencySamples, float64(latencyMs))
	}
	s.latencyCursor++
	if s.latencyCursor >= maxLatencySamples {
		s.latencyCursor = 0
		s.latencyFull = true
	}
	s.mu.Unlock()
}

// ActiveConnectionsInc increments the active connections counter.
func (s *Store) ActiveConnectionsInc() { s.activeConns.Add(1) }
func (s *Store) ActiveConnectionsDec() { s.activeConns.Add(-1) }

// SetActiveSessions updates the active stream session count (called from relay).
func (s *Store) SetActiveSessions(n int64) { s.activeSessions.Store(n) }

// SetStickyBoundSessions updates the sticky session count (called from balancer).
func (s *Store) SetStickyBoundSessions(n int64) { s.stickyBoundSessions.Store(n) }

// SetQuotaAlerts updates the quota alert count (called from alert evaluator).
func (s *Store) SetQuotaAlerts(n int64) { s.quotaAlerts.Store(n) }

// Snapshot returns a point-in-time snapshot of all runtime metrics.
func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	p95 := p95Locked(s.latencySamples)
	trend := make([]TrendPoint, len(s.trendSnapshots))
	copy(trend, s.trendSnapshots)
	hasTrend := len(s.trendSnapshots) > 0
	s.mu.Unlock()

	var avgLatencyMs float64
	totalReq := s.totalRequests.Load()
	if totalReq > 0 {
		avgLatencyMs = float64(s.totalWaitMs.Load()) / float64(totalReq)
	}

	var errorRate float64
	if totalReq > 0 {
		errorRate = float64(s.totalFailures.Load()) / float64(totalReq) * 100
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return Snapshot{
		TotalRequests:       totalReq,
		TotalFailures:       s.totalFailures.Load(),
		AvgLatencyMs:        avgLatencyMs,
		ErrorRate:           errorRate,
		P95LatencyMs:        p95,
		ThroughputRPS:       float64(s.throughputRPS.Load()),
		ActiveConnections:   s.activeConns.Load(),
		ActiveSessions:      s.activeSessions.Load(),
		StickyBoundSessions: s.stickyBoundSessions.Load(),
		QuotaAlerts:         s.quotaAlerts.Load(),
		MemoryMB:            int64(memStats.Alloc / 1024 / 1024),
		TrendSnapshots:      trend,
		HasTrendData:        hasTrend,
	}
}

// Snapshot holds a point-in-time view of all runtime metrics.
type Snapshot struct {
	TotalRequests       int64
	TotalFailures       int64
	AvgLatencyMs        float64
	ErrorRate           float64
	P95LatencyMs        float64
	ThroughputRPS       float64
	ActiveConnections   int64
	ActiveSessions      int64
	StickyBoundSessions int64
	QuotaAlerts         int64
	MemoryMB            int64
	TrendSnapshots      []TrendPoint
	HasTrendData        bool
}

func p95Locked(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)
	idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
