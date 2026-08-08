package modelnormalize

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

var builtinRouterPrefixes = []string{
	"dmxapi-",
	"agent-",
	"openai-",
	"anthropic-",
}

var builtinFunctionalSuffixes = []string{
	"-cc",
	"-fast",
	"-thinking",
	"-preview",
	"-beta",
	"-latest",
}

type ExplicitMapping struct {
	Variant   string `json:"variant"`
	Canonical string `json:"canonical"`
}

type Rules struct {
	RouterPrefixes     []string
	FunctionalSuffixes []string
	ExplicitMappings   []ExplicitMapping
	// explicitByKey 预处理缓存：dotDashKey(normalizeToBase(variant)) → canonical。
	// 同一 key 多条映射只保留第一条，自动消解用户的双向/冲突映射（存量数据也生效）。
	explicitByKey map[string]string
}

var rulesCache struct {
	mu    sync.RWMutex
	gen   uint64
	rules Rules
	ready bool
}

// suffixRegexCache 缓存编译后的正则后缀（pattern → *regexp.Regexp），
// 避免每次匹配都重新编译（模型广场 2886 个模型名 × 数十条正则后缀）。
var suffixRegexCache sync.Map

func Normalize(name string) string {
	return NormalizeWithRules(name, CurrentRules())
}

func CurrentRules() Rules {
	gen := setting.Generation()
	rulesCache.mu.RLock()
	if rulesCache.ready && rulesCache.gen == gen {
		rules := rulesCache.rules
		rulesCache.mu.RUnlock()
		return rules
	}
	rulesCache.mu.RUnlock()

	rules := loadRules()
	rulesCache.mu.Lock()
	rulesCache.gen = gen
	rulesCache.rules = rules
	rulesCache.ready = true
	rulesCache.mu.Unlock()
	return rules
}

func NormalizeWithRules(name string, rules Rules) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	// 显式映射：输入先完整规范化（剥路径+前缀+后缀）为基础名，再按
	// dotDashKey（-/. 统一、小写）查映射。同一 key 多条映射只取第一条，
	// 自动消解用户的双向/冲突映射（如 claude-opus-4-6 ↔ claude-opus-4.6），
	// 使同一模型不同命名归一到同一个 canonical。映射带任意渠道前缀/路径
	// 均与裸名变体互相命中。
	if len(rules.ExplicitMappings) > 0 {
		keyMap := rules.explicitByKey
		if keyMap == nil {
			keyMap = buildExplicitByKey(rules.ExplicitMappings, rules)
		}
		base := normalizeToBase(trimmed, rules)
		if canonical, ok := keyMap[dotDashKey(base)]; ok {
			return canonical
		}
	}

	result := stripPathAndRouterPrefix(trimmed, rules)
	result = stripFunctionalSuffixes(result, rules)
	return strings.ToLower(result)
}

// dotDashKey 把 - 和 . 统一为 . 并小写，用于显式映射的等价匹配与冲突检测。
// claude-opus-4-6 与 claude-opus-4.6 得到相同 key（同一模型两种命名），
// 而 gemini-2-5-pro 与 gemini-25-pro 得到不同 key（不同模型，不误并）。
func dotDashKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.ReplaceAll(s, "-", ".")
}

// buildExplicitByKey 预处理显式映射：key = dotDashKey(normalizeToBase(variant))，
// value = canonical（小写）。同一 key 只保留第一条（按用户配置顺序），
// 运行时据此自动忽略反向/冲突映射。
func buildExplicitByKey(mappings []ExplicitMapping, rules Rules) map[string]string {
	m := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		key := dotDashKey(normalizeToBase(mapping.Variant, rules))
		canonical := strings.ToLower(strings.TrimSpace(mapping.Canonical))
		if key == "" || canonical == "" {
			continue
		}
		if _, exists := m[key]; !exists {
			m[key] = canonical
		}
	}
	return m
}

// normalizeToBase 完整规范化：剥路径 + 路由前缀 + 功能性后缀，返回小写基础名。
func normalizeToBase(name string, rules Rules) string {
	result := stripPathAndRouterPrefix(name, rules)
	result = stripFunctionalSuffixes(result, rules)
	return strings.ToLower(result)
}

// stripFunctionalSuffixes 循环剥离功能性后缀。
// 每个后缀的匹配候选：
//  1. 字面原样（如 -cc 匹配结尾 -cc）；
//  2. -: 开头 → : 形式（-:free 匹配结尾 :free）；
//  3. -( 开头 → ( 形式（-(free) 匹配结尾 (free)）；
//  4. 含正则元字符（\d \w { } * + ? |）→ 编译为正则并锚定结尾
//     （-\d{8} 匹配 -20250514 这类日期后缀）。
func stripFunctionalSuffixes(name string, rules Rules) string {
	result := name
	for changed := true; changed; {
		changed = false
		lower := strings.ToLower(result)
		for _, suffix := range activeFunctionalSuffixes(rules) {
			suffix = strings.TrimSpace(suffix)
			if suffix == "" {
				continue
			}
			if n, ok := matchSuffixCandidate(lower, suffix); ok && n > 0 && len(result) > n {
				result = result[:len(result)-n]
				changed = true
				break
			}
		}
	}
	return result
}

// matchSuffixCandidate 尝试字面变体与正则变体，返回匹配的字符数。
func matchSuffixCandidate(lower, suffix string) (int, bool) {
	// 字面候选：原样、-: → :、-( → (。lower 已小写，候选也需小写才能命中
	// 用户配置的含大写后缀（如 -Thinking）。
	for _, cand := range literalSuffixCandidates(strings.ToLower(suffix)) {
		if strings.HasSuffix(lower, cand) && len(lower) > len(cand) {
			return len(cand), true
		}
	}
	// 正则候选：仅当含正则元字符，避免把 (free) 这类字面当作分组。
	if isRegexSuffix(suffix) {
		for _, cand := range regexSuffixCandidates(suffix) {
			re := getSuffixRegex(cand)
			if re == nil {
				continue
			}
			// loc[0] == len(lower) 表示只匹配到空串（如 -*、(x)? 这类可空 pattern），
			// 剥离长度为 0，放行会让外层循环永不收敛。
			if loc := re.FindStringIndex(lower); loc != nil && loc[0] < len(lower) {
				return len(lower) - loc[0], true
			}
		}
	}
	return 0, false
}

// getSuffixRegex 返回锚定结尾的正则（带编译缓存）。
// pattern 包进 (?:...) 再锚定，避免含 | 的 pattern（如 -32k|-64k）只有
// 最后一个分支被 $ 锚定、其余分支从字符串中间命中；(?i) 与字面候选的
// 大小写不敏感语义保持一致（待匹配串已小写）。
func getSuffixRegex(pattern string) *regexp.Regexp {
	if v, ok := suffixRegexCache.Load(pattern); ok {
		return v.(*regexp.Regexp)
	}
	re, err := regexp.Compile("(?i)(?:" + pattern + ")$")
	if err != nil {
		return nil
	}
	actual, _ := suffixRegexCache.LoadOrStore(pattern, re)
	return actual.(*regexp.Regexp)
}

// literalSuffixCandidates 生成字面后缀候选：原样、-: 前缀 → :、-( 前缀 → (。
func literalSuffixCandidates(suffix string) []string {
	cands := []string{suffix}
	if strings.HasPrefix(suffix, "-:") {
		cands = append(cands, ":"+suffix[2:])
	}
	if strings.HasPrefix(suffix, "-(") {
		cands = append(cands, "("+suffix[2:])
	}
	return cands
}

// regexSuffixCandidates 生成正则后缀候选：原样、-@ 开头 → @ 变体
// （-@\w+ → @\w+，覆盖 claude-3-haiku@20240307 这类无连字符 @ 变体）。
// 其余正则（如 -\d{8}）只按原样匹配，避免变体 \d{8} 误剥 @20240307 这类无 - 前缀的日期。
func regexSuffixCandidates(suffix string) []string {
	cands := []string{suffix}
	if strings.HasPrefix(suffix, "-@") {
		cands = append(cands, suffix[1:])
	}
	return cands
}

// isRegexSuffix 判断后缀是否为「显式正则」：含 \d \w { } * + ? | 之一。
// 排除 ( ) [ ] - 等常见字面字符，避免 -(free) 被误当分组。
func isRegexSuffix(suffix string) bool {
	return strings.ContainsAny(suffix, `\d\w{}*+?|`)
}

// stripPathAndRouterPrefix 剥离路径前缀（最后一个 / 之后）与路由前缀，返回中间名。
// 前缀匹配支持「去尾 - 变体」：用户常把 [官B]- 写成带连字符，但渠道模型名是
// [官B]claude-opus-4-6（[官B] 后无连字符），原样匹配不上；去尾 - 变体覆盖该写法差异。
func stripPathAndRouterPrefix(name string, rules Rules) string {
	result := name
	if slashIndex := strings.LastIndex(result, "/"); slashIndex >= 0 {
		result = result[slashIndex+1:]
	}

	lower := strings.ToLower(result)
	for _, prefix := range activeRouterPrefixes(rules) {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		// 原样匹配（dmxapi-kimi-k2.5 → dmxapi-）。
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			result = result[len(prefix):]
			break
		}
		// 去尾 - 变体（[官B]- → [官B] 匹配 [官B]claude-opus-4-6）。
		// 仅当 base 以非字母数字结尾（]、) 等括号类标记）才启用：
		// agent-/anthropic- 这类字母结尾的前缀去尾后会从单词中间误剥
		// （agentic-coder → ic-coder、anthropic.claude-x → .claude-x）。
		if strings.HasSuffix(prefix, "-") {
			base := strings.TrimSuffix(prefix, "-")
			if base != "" && !endsWithAlphanumeric(base) && strings.HasPrefix(lower, strings.ToLower(base)) {
				result = result[len(base):]
				// 若原名字带 -（dmxapi-kimi → 剥 dmxapi 剩 -kimi），去掉。
				result = strings.TrimLeft(result, "-")
				break
			}
		}
	}
	return result
}

// endsWithAlphanumeric 报告字符串最后一个 rune 是否为字母或数字。
func endsWithAlphanumeric(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}
	last := runes[len(runes)-1]
	return unicode.IsLetter(last) || unicode.IsDigit(last)
}

func loadRules() Rules {
	rules := Rules{
		RouterPrefixes:     loadStringArray(model.SettingKeyModelNormalizeRouterPrefixes),
		FunctionalSuffixes: loadStringArray(model.SettingKeyModelNormalizeFunctionalSuffixes),
		ExplicitMappings:   loadExplicitMappings(model.SettingKeyModelNormalizeExplicitMappings),
	}
	// 预构建显式映射索引：NormalizeWithRules 是市场热路径（每模型名调用一次），
	// 若每次调用都 buildExplicitByKey（O(映射数)），模型名上万 × 映射上百时
	// 单次市场聚合可达秒级，是网关 504 的候选成因。此处缓存刷新时构建一次。
	rules.explicitByKey = buildExplicitByKey(rules.ExplicitMappings, rules)
	return rules
}

func activeRouterPrefixes(rules Rules) []string {
	if len(rules.RouterPrefixes) > 0 {
		return rules.RouterPrefixes
	}
	return builtinRouterPrefixes
}

func activeFunctionalSuffixes(rules Rules) []string {
	if len(rules.FunctionalSuffixes) > 0 {
		return rules.FunctionalSuffixes
	}
	return builtinFunctionalSuffixes
}

func loadStringArray(key model.SettingKey) []string {
	raw, err := setting.GetString(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return compactStrings(values)
}

func loadExplicitMappings(key model.SettingKey) []ExplicitMapping {
	raw, err := setting.GetString(key)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []ExplicitMapping
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	result := make([]ExplicitMapping, 0, len(values))
	for _, value := range values {
		variant := strings.TrimSpace(value.Variant)
		canonical := strings.TrimSpace(value.Canonical)
		if variant == "" || canonical == "" {
			continue
		}
		result = append(result, ExplicitMapping{Variant: variant, Canonical: canonical})
	}
	return result
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
