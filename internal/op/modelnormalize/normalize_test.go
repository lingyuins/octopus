package modelnormalize

import (
	"testing"
	"time"
)

func TestNormalizeWithRules_UsesBuiltinRules(t *testing.T) {
	rules := Rules{}
	tests := map[string]string{
		"kimi-k2.5":                "kimi-k2.5",
		"@cf/moonshotai/kimi-k2.5": "kimi-k2.5",
		"dmxapi-kimi-k2.5":         "kimi-k2.5",
		"moonshotai/kimi-k2.5":     "kimi-k2.5",
		"agent/kimi-k2.5":          "kimi-k2.5",
		"kimi-k2.5-cc":             "kimi-k2.5",
		"Kimi-K2.5-CC":             "kimi-k2.5",
		"kimi-k2.5-preview-fast":   "kimi-k2.5",
	}

	for input, want := range tests {
		if got := NormalizeWithRules(input, rules); got != want {
			t.Fatalf("NormalizeWithRules(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeWithRules_ExplicitMappingTakesPrecedence(t *testing.T) {
	rules := Rules{
		ExplicitMappings: []ExplicitMapping{{Variant: "dmxapi-kimi-k2.5-cc", Canonical: "kimi-k2.5-routing"}},
	}

	if got := NormalizeWithRules("DMXAPI-KIMI-K2.5-CC", rules); got != "kimi-k2.5-routing" {
		t.Fatalf("NormalizeWithRules explicit = %q, want kimi-k2.5-routing", got)
	}
}

// 显式映射应匹配「剥离路径 + 路由前缀后的基础名」：用户导入裸名 variant，
// 渠道侧模型名带前缀/路径时也能命中（dmxapi-kimi-k2.5-256k → kimi-k2.5-256k）。
func TestNormalizeWithRules_ExplicitMappingMatchesStrippedBase(t *testing.T) {
	rules := Rules{
		ExplicitMappings: []ExplicitMapping{{Variant: "kimi-k2.5-256k", Canonical: "kimi-k2.5"}},
	}

	tests := map[string]string{
		"kimi-k2.5-256k":            "kimi-k2.5", // 裸名精确匹配
		"dmxapi-kimi-k2.5-256k":     "kimi-k2.5", // 路由前缀剥离后命中
		"moonshotai/kimi-k2.5-256k": "kimi-k2.5", // 路径剥离后命中
		"@cf/org/kimi-k2.5-256k":    "kimi-k2.5", // 多级路径
	}
	for input, want := range tests {
		if got := NormalizeWithRules(input, rules); got != want {
			t.Fatalf("NormalizeWithRules(%q) = %q, want %q", input, got, want)
		}
	}
}

// 显式映射命中后不误伤：带前缀但基础名不同的变体不应被错误合并。
func TestNormalizeWithRules_ExplicitMappingDoesNotOverMatch(t *testing.T) {
	rules := Rules{
		ExplicitMappings: []ExplicitMapping{{Variant: "kimi-k2.5-256k", Canonical: "kimi-k2.5"}},
	}
	// 基础名是 kimi-k2.5（非 256k 变体），不应命中显式映射；走内置规则应保持独立。
	if got := NormalizeWithRules("dmxapi-kimi-k2.5", rules); got != "kimi-k2.5" {
		t.Fatalf("NormalizeWithRules(dmxapi-kimi-k2.5) = %q, want kimi-k2.5", got)
	}
	// 另一个模型的 256k 变体不应被映射到 kimi。
	if got := NormalizeWithRules("dmxapi-gpt-4o-256k", rules); got == "kimi-k2.5" {
		t.Fatal("gpt-4o-256k 被错误映射到 kimi-k2.5")
	}
}

func TestNormalizeWithRules_RuntimeRulesOverrideBuiltinPrefixesAndSuffixes(t *testing.T) {
	rules := Rules{
		RouterPrefixes:     []string{"custom-"},
		FunctionalSuffixes: []string{"-route"},
	}

	if got := NormalizeWithRules("dmxapi-kimi-k2.5-cc", rules); got != "dmxapi-kimi-k2.5-cc" {
		t.Fatalf("NormalizeWithRules with overridden rules = %q, want original lower-case", got)
	}
	if got := NormalizeWithRules("custom-kimi-k2.5-route", rules); got != "kimi-k2.5" {
		t.Fatalf("NormalizeWithRules custom = %q, want kimi-k2.5", got)
	}
}

// 可匹配空串的正则后缀（-*、(x)? 等）不应导致 stripFunctionalSuffixes 死循环。
func TestNormalizeWithRules_EmptyMatchRegexSuffixTerminates(t *testing.T) {
	for _, suffix := range []string{"-*", "(x)?", "-32k|"} {
		done := make(chan string, 1)
		go func() {
			done <- NormalizeWithRules("gpt-4o", Rules{FunctionalSuffixes: []string{suffix}})
		}()
		select {
		case got := <-done:
			if got != "gpt-4o" {
				t.Fatalf("suffix %q: NormalizeWithRules(gpt-4o) = %q, want gpt-4o", suffix, got)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("suffix %q: NormalizeWithRules 死循环（3s 未返回）", suffix)
		}
	}
}

// 含 | 的正则后缀每个分支都应锚定结尾，不得从字符串中间剥离。
func TestNormalizeWithRules_AlternationSuffixAnchorsAllBranches(t *testing.T) {
	rules := Rules{FunctionalSuffixes: []string{"-32k|-64k"}}
	if got := NormalizeWithRules("qwen-32k-chat", rules); got != "qwen-32k-chat" {
		t.Fatalf("NormalizeWithRules(qwen-32k-chat) = %q, want qwen-32k-chat（-32k 在中间，不应剥离）", got)
	}
	if got := NormalizeWithRules("qwen-turbo-32k", rules); got != "qwen-turbo" {
		t.Fatalf("NormalizeWithRules(qwen-turbo-32k) = %q, want qwen-turbo", got)
	}
	if got := NormalizeWithRules("qwen-turbo-64k", rules); got != "qwen-turbo" {
		t.Fatalf("NormalizeWithRules(qwen-turbo-64k) = %q, want qwen-turbo", got)
	}
}

// 字母结尾前缀的去尾 - 变体不应从单词中间误剥；括号类前缀保持生效。
func TestNormalizeWithRules_TrailingDashVariantRequiresBoundary(t *testing.T) {
	if got := Normalize("agentic-coder"); got != "agentic-coder" {
		t.Fatalf("Normalize(agentic-coder) = %q, want agentic-coder", got)
	}
	if got := Normalize("anthropic.claude-sonnet-4-5"); got != "anthropic.claude-sonnet-4-5" {
		t.Fatalf("Normalize(anthropic.claude-sonnet-4-5) = %q, want anthropic.claude-sonnet-4-5", got)
	}
	// [官B]- 写法仍应命中 [官B]claude-opus-4-6（无连字符）。
	rules := Rules{RouterPrefixes: []string{"[官B]-"}}
	if got := NormalizeWithRules("[官B]claude-opus-4-6", rules); got != "claude-opus-4-6" {
		t.Fatalf("NormalizeWithRules([官B]claude-opus-4-6) = %q, want claude-opus-4-6", got)
	}
}

// 后缀匹配保持大小写不敏感（与旧版行为一致）。
func TestNormalizeWithRules_SuffixMatchingCaseInsensitive(t *testing.T) {
	rules := Rules{FunctionalSuffixes: []string{"-Thinking"}}
	if got := NormalizeWithRules("claude-x-thinking", rules); got != "claude-x" {
		t.Fatalf("字面后缀大小写: got %q, want claude-x", got)
	}
	rules = Rules{FunctionalSuffixes: []string{`-V\d+`}}
	if got := NormalizeWithRules("model-v20250514", rules); got != "model" {
		t.Fatalf("正则后缀大小写: got %q, want model", got)
	}
}
