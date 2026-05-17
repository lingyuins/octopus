package op

import "testing"

func TestRelayLogCacheReadTokens_ParsesProviderPromptCacheUsage(t *testing.T) {
	got := relayLogCacheReadTokens(`{"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":320},"output_tokens":10}}`)
	if got != 320 {
		t.Fatalf("relayLogCacheReadTokens() = %d, want 320", got)
	}
}

func TestRelayLogCacheReadTokens_ParsesPromptTokensDetailsCachedTokens(t *testing.T) {
	got := relayLogCacheReadTokens(`{"usage":{"prompt_tokens":254,"prompt_tokens_details":{"cached_tokens":192},"output_tokens":161}}`)
	if got != 192 {
		t.Fatalf("relayLogCacheReadTokens() = %d, want 192", got)
	}
}

func TestRelayLogCacheReadTokens_ParsesPromptCacheHitTokens(t *testing.T) {
	got := relayLogCacheReadTokens(`{"usage":{"prompt_tokens":254,"prompt_cache_hit_tokens":144,"output_tokens":161}}`)
	if got != 144 {
		t.Fatalf("relayLogCacheReadTokens() = %d, want 144", got)
	}
}

func TestRelayLogCacheReadTokens_ParsesTopLevelUsageCachedTokens(t *testing.T) {
	got := relayLogCacheReadTokens(`{"usage":{"input_tokens":1000,"cached_tokens":280,"output_tokens":10}}`)
	if got != 280 {
		t.Fatalf("relayLogCacheReadTokens() = %d, want 280", got)
	}
}

func TestRelayLogCacheReadTokens_ReturnsZeroWithoutCachedTokens(t *testing.T) {
	tests := []string{
		``,
		`{"usage":{"input_tokens":1000}}`,
		`{"usage":{"input_tokens":1000,"input_tokens_details":{"cached_tokens":0}}}`,
		`not-json`,
	}

	for _, input := range tests {
		if got := relayLogCacheReadTokens(input); got != 0 {
			t.Fatalf("relayLogCacheReadTokens(%q) = %d, want 0", input, got)
		}
	}
}
