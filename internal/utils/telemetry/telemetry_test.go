package telemetry

import "testing"

func TestRecordRequestP95IgnoresUninitializedSamples(t *testing.T) {
	store := NewStore()
	store.RecordRequest(100, true)
	store.RecordRequest(200, true)
	store.RecordRequest(300, true)

	snap := store.Snapshot()
	if snap.P95LatencyMs != 300 {
		t.Fatalf("P95LatencyMs = %v, want 300", snap.P95LatencyMs)
	}
	if snap.AvgLatencyMs != 200 {
		t.Fatalf("AvgLatencyMs = %v, want 200", snap.AvgLatencyMs)
	}
}
