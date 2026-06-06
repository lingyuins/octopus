 docs/memory-leak-troubleshooting
package semantic_cache

import (
	"bytes"
	"testing"
	"time"
)

func TestLookup_IsolatedByNamespace(t *testing.T) {
	Reset()
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	Store("k1:chat:gpt-4.1", "req-a", []byte(`{"id":"resp-a"}`), embedding)

	if _, ok := Lookup("k2:chat:gpt-4.1", embedding); ok {
		t.Fatal("expected namespace miss")
	}
}

func TestStore_CopiesResponseJSON(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	responseJSON := []byte(`{"id":"resp-a"}`)
	Store("k1:chat:gpt-4.1", "req-a", responseJSON, embedding)

	copy(responseJSON, []byte(`{"id":"mutate"}`))

	got, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !bytes.Equal(got, []byte(`{"id":"resp-a"}`)) {
		t.Fatalf("Lookup() = %s, want original response", string(got))
	}
}

func TestLookup_ReturnsResponseCopy(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	Store("k1:chat:gpt-4.1", "req-a", []byte(`{"id":"resp-a"}`), embedding)

	got, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected first cache hit")
	}
	copy(got, []byte(`{"id":"mutate"}`))

	gotAgain, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected second cache hit")
	}
	if !bytes.Equal(gotAgain, []byte(`{"id":"resp-a"}`)) {
		t.Fatalf("Lookup() after caller mutation = %s, want original response", string(gotAgain))
	}
}

func TestApplyRuntimeConfig_PreservesEntriesWhenUnchanged(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	cfg := RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour}
	ApplyRuntimeConfig(cfg)

	embedding := []float64{1, 0}
	Store("k1:chat:gpt-4.1", "req-a", []byte(`{"id":"resp-a"}`), embedding)

	ApplyRuntimeConfig(cfg)

	got, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected cache hit after applying unchanged runtime config")
	}
	if !bytes.Equal(got, []byte(`{"id":"resp-a"}`)) {
		t.Fatalf("Lookup() after unchanged config = %s, want original response", string(got))
	}
}

package semantic_cache

import (
	"bytes"
	"testing"
	"time"
)

func TestLookup_IsolatedByNamespace(t *testing.T) {
	Reset()
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	Store("k1:chat:gpt-4.1", "req-a", []byte(`{"id":"resp-a"}`), embedding)

	if _, ok := Lookup("k2:chat:gpt-4.1", embedding); ok {
		t.Fatal("expected namespace miss")
	}
}

func TestStore_CopiesResponseJSON(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	responseJSON := []byte(`{"id":"resp-a"}`)
	Store("k1:chat:gpt-4.1", "req-a", responseJSON, embedding)

	copy(responseJSON, []byte(`{"id":"mutate"}`))

	got, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !bytes.Equal(got, []byte(`{"id":"resp-a"}`)) {
		t.Fatalf("Lookup() = %s, want original response", string(got))
	}
}

func TestLookup_ReturnsResponseCopy(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: time.Hour})

	embedding := []float64{1, 0}
	Store("k1:chat:gpt-4.1", "req-a", []byte(`{"id":"resp-a"}`), embedding)

	got, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected first cache hit")
	}
	copy(got, []byte(`{"id":"mutate"}`))

	gotAgain, ok := Lookup("k1:chat:gpt-4.1", embedding)
	if !ok {
		t.Fatal("expected second cache hit")
	}
	if !bytes.Equal(gotAgain, []byte(`{"id":"resp-a"}`)) {
		t.Fatalf("Lookup() after caller mutation = %s, want original response", string(gotAgain))
	}
}

func TestApplyRuntimeConfig_PreservesEntriesWhenUnchanged(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cfg := RuntimeConfig{
		Enabled:    true,
		MaxEntries: 16,
		Threshold:  0.95,
		TTL:        time.Hour,
	}
	ApplyRuntimeConfig(cfg)

	embedding := []float64{1, 0}
	Store("ns", "req-1", []byte(`{"id":"resp-1"}`), embedding)
	Store("ns", "req-2", []byte(`{"id":"resp-2"}`), []float64{0, 1})

	_, _, sizeBefore := Stats()
	if sizeBefore != 2 {
		t.Fatalf("expected 2 entries before re-apply, got %d", sizeBefore)
	}

	// Re-apply identical config — entries must be preserved.
	ApplyRuntimeConfig(cfg)

	_, _, sizeAfter := Stats()
	if sizeAfter != 2 {
		t.Fatalf("expected 2 entries after re-apply with same config, got %d", sizeAfter)
	}

	// Verify the stored data is still retrievable.
	if _, ok := Lookup("ns", embedding); !ok {
		t.Fatal("expected cache hit after re-apply with same config")
	}
}

func TestApplyRuntimeConfig_ClearsEntriesWhenChanged(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cfg := RuntimeConfig{
		Enabled:    true,
		MaxEntries: 16,
		Threshold:  0.95,
		TTL:        time.Hour,
	}
	ApplyRuntimeConfig(cfg)

	Store("ns", "req-1", []byte(`{"id":"resp-1"}`), []float64{1, 0})

	// Change MaxEntries — should rebuild cache.
	cfg.MaxEntries = 32
	ApplyRuntimeConfig(cfg)

	_, _, sizeAfter := Stats()
	if sizeAfter != 0 {
		t.Fatalf("expected 0 entries after config change, got %d", sizeAfter)
	}
}

func TestApplyRuntimeConfig_DisabledClearsCache(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	ApplyRuntimeConfig(RuntimeConfig{
		Enabled:    true,
		MaxEntries: 16,
		Threshold:  0.95,
		TTL:        time.Hour,
	})
	Store("ns", "req-1", []byte(`{"id":"resp-1"}`), []float64{1, 0})

	ApplyRuntimeConfig(RuntimeConfig{Enabled: false})

	if Enabled() {
		t.Fatal("expected cache to be disabled after applying Enabled=false")
	}
}

func TestStats_PruneExpiredEntries(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ApplyRuntimeConfig(RuntimeConfig{Enabled: true, MaxEntries: 16, Threshold: 0.95, TTL: 50 * time.Millisecond})

	Store("ns", "req-1", []byte(`{"id":"resp-1"}`), []float64{1, 0})
	Store("ns", "req-2", []byte(`{"id":"resp-2"}`), []float64{0, 1})

	_, _, sizeBefore := Stats()
	if sizeBefore != 2 {
		t.Fatalf("expected 2 entries before TTL, got %d", sizeBefore)
	}

	time.Sleep(100 * time.Millisecond)

	_, _, sizeAfter := Stats()
	if sizeAfter != 0 {
		t.Fatalf("expected 0 entries after TTL expiry, got %d", sizeAfter)
	}
}
 master
