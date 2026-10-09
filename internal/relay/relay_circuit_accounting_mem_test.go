package relay

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 源码文本守卫（*_mem_test.go 既有模式，参考 op/ops/ops_mem_test.go）。
//
// 这组断言锁住运行时难以验证、但一旦被回退就会重新引入生产故障的三件事：
//  1. relay.go 的熔断守卫必须走 shouldRecordChannelFailure，不能退回裸 Scope 判定；
//  2. media_relay.go 必须同样带断连豁免，防止两文件逻辑漂移；
//  3. 号池失败块里 poolscheduler.ReleaseSlot 必须仍无条件执行——
//     这是本次最重要的守卫：client disconnected 早退块绝不能被前移到它之前，
//     否则号池并发槽位泄漏（globalPoolSlots 是纯内存 sync.Map，默认
//     EffectiveConcurrency 为 1，单次泄漏即完全饱和；PurgeStale 遍历的是
//     globalPoolStats 而非 globalPoolSlots，存在结构性盲区，救不回来）。

func readRelaySourceFile(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src, err := os.ReadFile(filepath.Clean(filepath.Join(filepath.Dir(file), name)))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	// 本仓工作区在 core.autocrlf=true 下检出为 CRLF，而下方多处守卫用
	// "\n" 做边界匹配（切取函数体、反向守卫多行字面量）。统一归一为 LF，
	// 否则这些守卫会在 CRLF 工作区里静默失效（永不命中）。
	return strings.ReplaceAll(string(src), "\r\n", "\n")
}

// stripGoComments 去掉整行注释与行尾注释，保留代码结构。
//
// 源码文本守卫必须针对代码而非注释：本仓在号池块上方写了大段解释取舍的中文注释，
// 其中就包含 SkipFailureAccounting / ReleaseSlot 这些字样，若不剥离注释，
// 守卫会被注释自身满足（假阳性）或被注释误触（假阴性）。
// 行尾注释仅在行内不含字符串字面量时剥离，避免误伤包含 "//" 的字符串（如 URL）。
func stripGoComments(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue // 整行注释
		}
		if idx := strings.Index(line, "//"); idx >= 0 {
			head := line[:idx]
			if !strings.Contains(head, `"`) && !strings.Contains(head, "`") {
				line = head
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// readRelayCode 读取并归一化源文件（LF + 去注释），用于结构性守卫。
func readRelayCode(t *testing.T, name string) string {
	t.Helper()
	return stripGoComments(readRelaySourceFile(t, name))
}

func TestRelayGoUsesShouldRecordChannelFailureGuard(t *testing.T) {
	src := readRelayCode(t, "relay.go")

	if !strings.Contains(src, "shouldRecordChannelFailure(channel.PoolID, result.Decision)") {
		t.Fatal("relay.go 必须用 shouldRecordChannelFailure(channel.PoolID, result.Decision) 作为熔断守卫；" +
			"裸 `channel.PoolID == 0 && (Scope == ScopeNextChannel || Scope == ScopeAbortAll)` 会把客户端断连" +
			"（Decision 恰为 ScopeAbortAll）计入熔断器，导致健康渠道被误熔断")
	}

	// 反向守卫：旧的裸判定不得回来。
	legacy := "channel.PoolID == 0 && (result.Decision.Scope == ScopeNextChannel || result.Decision.Scope == ScopeAbortAll)"
	if strings.Contains(src, legacy) {
		t.Fatalf("relay.go 仍含旧的裸 Scope 熔断守卫：%q", legacy)
	}
}

func TestMediaRelayGoHasDisconnectExemption(t *testing.T) {
	src := readRelayCode(t, "media_relay.go")
	raw := readRelaySourceFile(t, "media_relay.go")

	// SkipFailureAccounting 在媒体侧只出现于解释豁免语义的注释里：实际写入该字段
	// 的代码在 type.go 的 markClientCancelIfGone 中。所以这一条查原始文本
	//（防两文件语义漂移：媒体侧必须明确记录断连豁免的存在），
	// 下面两条查去注释后的代码（防真正的接线被拆掉）。
	if !strings.Contains(raw, "SkipFailureAccounting") {
		t.Fatal("media_relay.go 必须包含 SkipFailureAccounting 断连豁免，否则与 relay.go 漂移：" +
			"媒体侧写失败会把 statusCode 置 0，ClassifyRelayError 只能归为 ScopeNextChannel 或 " +
			"ScopeAbortAll，把客户端主动停止误判成上游故障")
	}
	if !strings.Contains(src, "markClientCancelIfGone(&decision, c.Request.Context(), fwdErr)") {
		t.Fatal("media_relay.go 必须调 markClientCancelIfGone 并传 c.Request.Context()；" +
			"operationCtx 基于 context.Background()（context.go），查它永远看不到客户端断连")
	}
	if !strings.Contains(src, "shouldRecordChannelFailure(") {
		t.Fatal("media_relay.go 必须复用 shouldRecordChannelFailure 守卫，避免两文件判定逻辑漂移")
	}

	// 反向守卫：媒体侧不得退回裸 Scope 判定。
	legacy := "if decision.Scope == ScopeNextChannel || decision.Scope == ScopeAbortAll {\n\t\t\t\t\tbalancer.RecordFailure"
	if strings.Contains(src, legacy) {
		t.Fatal("media_relay.go 仍含旧的裸 Scope 熔断守卫，应改用 shouldRecordChannelFailure")
	}
}

// 本次最重要的回归守卫：号池并发槽位不得泄漏。
func TestRelayGoPoolBlockReleasesSlotUnconditionally(t *testing.T) {
	src := readRelayCode(t, "relay.go")

	const (
		reportResult = "poolscheduler.ReportResult(channel.PoolID, poolAccount.ID, false, 0, 0)"
		releaseSlot  = "poolscheduler.ReleaseSlot(channel.PoolID, poolAccount.ID)"
	)

	reportIdx := strings.Index(src, reportResult)
	if reportIdx < 0 {
		t.Fatalf("relay.go 的号池失败块缺少 %q", reportResult)
	}
	releaseIdx := strings.Index(src[reportIdx:], releaseSlot)
	if releaseIdx < 0 {
		t.Fatalf("relay.go 的号池失败块在 ReportResult 之后缺少无条件 %q；"+
			"缺了它号池并发槽位会泄漏", releaseSlot)
	}
	releaseIdx += reportIdx

	// ReleaseSlot 必须紧跟 ReportResult，且两者之间不得出现任何条件语句——
	// 也就是它不能被包进 if（含 SkipFailureAccounting 判定）里。
	between := src[reportIdx+len(reportResult) : releaseIdx]
	for _, banned := range []string{"if ", "else", "return"} {
		if strings.Contains(between, banned) {
			t.Fatalf("ReportResult 与 ReleaseSlot 之间出现了 %q（内容：%q）；"+
				"ReleaseSlot 必须无条件执行，否则断连路径会泄漏号池槽位", banned, strings.TrimSpace(between))
		}
	}

	// ReportResult 同样必须无条件（它同时是 globalPoolStats 条目与 lastActivity
	// 的唯一创建点，跳过会造成 PurgeStale 的结构性盲区）。
	// 守卫方式：ReportResult 之前不得出现针对它的 SkipFailureAccounting 条件。
	poolBlockStart := strings.LastIndex(src[:reportIdx], "if poolAccount != nil {")
	if poolBlockStart < 0 {
		t.Fatal("relay.go 找不到号池失败块入口 `if poolAccount != nil {`")
	}
	blockHead := src[poolBlockStart:reportIdx]
	if strings.Contains(blockHead, "SkipFailureAccounting") {
		t.Fatalf("ReportResult 之前出现了 SkipFailureAccounting 判定（%q）；"+
			"ReportResult 是 globalPoolStats 条目与 lastActivity 的唯一创建点，"+
			"条件化会造成 PurgeStale 的结构性盲区与槽位永久假死", strings.TrimSpace(blockHead))
	}

	// client disconnected 早退块必须位于 ReleaseSlot 之后：
	// 前移即跳过槽位释放。
	disconnectIdx := strings.Index(src[releaseIdx:], "errors.Is(result.Err, errClientDisconnected)")
	if disconnectIdx < 0 {
		t.Fatal("relay.go 在 ReleaseSlot 之后找不到 client disconnected 早退块；" +
			"它若被前移到 ReleaseSlot 之前，号池槽位会泄漏")
	}
}

// 契约的「声明侧」守卫：attempt() 的两个豁免分支必须真的带上
// SkipFailureAccounting: true。否则即使 shouldRecordChannelFailure 守卫写得再对，
// 断连与关键词拦截仍会被计入熔断——这正是本次修复的核心。
//
// 这里逐分支切片校验，而不是只数全文出现次数：只数次数的话，
// 把标记从断连分支挪到别的分支也能让测试通过。
func TestRelayGoAttemptDeclaresSkipFailureAccounting(t *testing.T) {
	src := readRelayCode(t, "relay.go")

	// decisionSlice 切出从分支入口到其 attemptResult 构造结束之间的代码。
	decisionSlice := func(marker string) string {
		start := strings.Index(src, marker)
		if start < 0 {
			t.Fatalf("relay.go 的 attempt() 缺少分支 %q", marker)
		}
		rest := src[start:]
		end := strings.Index(rest, "attemptResult{")
		if end < 0 {
			t.Fatalf("分支 %q 之后找不到 attemptResult 构造", marker)
		}
		closeBrace := strings.Index(rest[end:], "\n\t\t}\n")
		if closeBrace < 0 {
			t.Fatalf("分支 %q 的 attemptResult 构造未正常结束", marker)
		}
		return rest[:end+closeBrace]
	}

	for _, tc := range []struct {
		marker string
		what   string
	}{
		{marker: "if errors.Is(fwdErr, errClientDisconnected) {", what: "客户端断连"},
		{marker: "if errors.Is(fwdErr, errResponseFilterBlocked) {", what: "关键词拦截"},
	} {
		slice := decisionSlice(tc.marker)
		if !strings.Contains(slice, "SkipFailureAccounting: true") {
			t.Fatalf("attempt() 的 %s 分支（%s）缺少 SkipFailureAccounting: true；"+
				"少了它，shouldRecordChannelFailure 守卫无从识别豁免，%s会被当成渠道故障计入熔断器",
				tc.what, tc.marker, tc.what)
		}
	}

	// 反向守卫：空输出重试分支用的是 ScopeSameChannel（天然不进守卫），
	// 不应也不需要带 SkipFailureAccounting；若将来有人给它加上，
	// 说明对守卫语义的理解已经漂移，需要重新评估。
	emptyStart := strings.Index(src, "if errors.Is(fwdErr, errEmptyOutput) {")
	if emptyStart >= 0 {
		rest := src[emptyStart:]
		if end := strings.Index(rest, "attemptResult{"); end >= 0 {
			if closeBrace := strings.Index(rest[end:], "\n\t\t}\n"); closeBrace >= 0 {
				if strings.Contains(rest[:end+closeBrace], "SkipFailureAccounting") {
					t.Fatal("空输出重试分支（ScopeSameChannel）不应带 SkipFailureAccounting；" +
						"它本来就不进熔断守卫，加上意味着对守卫语义的理解已漂移")
				}
			}
		}
	}
}

// 关键词拦截哨兵必须用 %w 包裹，否则 attempt() 的 errors.Is 分支是不可达死码。
func TestRelayGoWrapsResponseFilterSentinel(t *testing.T) {
	src := readRelayCode(t, "relay.go")

	const wantReturn = `return fmt.Errorf("response filter blocked streaming output: %w", errResponseFilterBlocked)`
	if !strings.Contains(src, wantReturn) {
		t.Fatalf("relay.go 缺少 %q；裸 fmt.Errorf 会丢失哨兵，"+
			"使 attempt() 的 errors.Is(fwdErr, errResponseFilterBlocked) 分支不可达，"+
			"拦截失败退化成普通 transformer 错误 → 记熔断 + 换渠道重试", wantReturn)
	}

	// 反向守卫：不得出现丢哨兵的裸写法。
	const bareReturn = `return fmt.Errorf("response filter blocked streaming output")`
	if strings.Contains(src, bareReturn) {
		t.Fatalf("relay.go 仍含丢哨兵的裸写法：%q", bareReturn)
	}

	// 两个消费点必须仍然是 errors.Is 判定。
	if strings.Count(src, "errors.Is(fwdErr, errResponseFilterBlocked)") < 1 {
		t.Fatal("attempt() 缺少 errors.Is(fwdErr, errResponseFilterBlocked) 消费点")
	}
	if strings.Count(src, "errors.Is(err, errResponseFilterBlocked)") < 1 {
		t.Fatal("handleStreamResponse 缺少 errors.Is(err, errResponseFilterBlocked) 消费点")
	}
}

// [DONE] 早退必须存在，且必须与 EOF 收尾共用 finalizeStream（不得复制一份逻辑，
// 否则 issue #155 的空输出重试语义会在两条路径上漂移）。
func TestRelayGoHandlesSSEDoneMarkerViaSharedFinalize(t *testing.T) {
	src := readRelayCode(t, "relay.go")

	if !strings.Contains(src, "isSSEDoneMarker(r.data)") {
		t.Fatal("relay.go 必须用 isSSEDoneMarker 识别上游的 [DONE] 标记；" +
			"缺少它时，发完 [DONE] 却不关连接的上游会让请求阻塞到客户端超时断开，" +
			"被误记为 client disconnected")
	}
	if !strings.Contains(src, "streamFinished = internalStream.StreamFinished") {
		t.Fatal("relay.go must recognize protocol terminal events as well as [DONE]")
	}

	finalizeCalls := strings.Count(src, "return finalizeStream()")
	if finalizeCalls < 2 {
		t.Fatalf("relay.go 中 `return finalizeStream()` 出现 %d 次，want >= 2"+
			"（[DONE] 早退与 EOF 收尾必须共用同一个 finalizeStream，避免语义漂移）", finalizeCalls)
	}
	if !strings.Contains(src, "finalizeStream := func() error {") {
		t.Fatal("relay.go 缺少 finalizeStream 闭包定义")
	}

	// finalizeStream 内必须保留 issue #155 的空输出重试判定：
	// 收到 [DONE] 不等于有可见内容。
	finalizeStart := strings.Index(src, "finalizeStream := func() error {")
	finalizeBody := src[finalizeStart:]
	if end := strings.Index(finalizeBody, "\n\tfor {"); end > 0 {
		finalizeBody = finalizeBody[:end]
	}
	for _, required := range []string{
		"isRetryEmptyOutputEnabled()",
		"!hasVisibleContent",
		"return errEmptyOutput",
		"ra.streamSession.Finish(nil)",
		`ra.inAdapter.TransformStream(ctx, &model.InternalLLMResponse{Object: "[DONE]"})`,
	} {
		if !strings.Contains(finalizeBody, required) {
			t.Fatalf("finalizeStream 缺少 %q；[DONE] 早退会破坏 issue #155 的空输出重试语义"+
				"或 stream session 收尾", required)
		}
	}
}

// SkipFailureAccounting 字段与守卫函数必须存在于 type.go（防止有人把字段删掉
// 却留下调用点，或反之）。
func TestTypeGoDeclaresSkipFailureAccountingContract(t *testing.T) {
	src := readRelayCode(t, "type.go")

	for _, required := range []string{
		"SkipFailureAccounting bool",
		"func shouldRecordChannelFailure(poolID int, decision RetryDecision) bool",
		"func markClientCancelIfGone(decision *RetryDecision, clientCtx context.Context, err error)",
		"func isSSEDoneMarker(data string) bool",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("type.go 缺少 %q", required)
		}
	}

	// 守卫的三条判定缺一不可。
	guardBody := src[strings.Index(src, "func shouldRecordChannelFailure"):]
	if end := strings.Index(guardBody, "\n}"); end > 0 {
		guardBody = guardBody[:end]
	}
	for _, required := range []string{"poolID == 0", "!decision.SkipFailureAccounting", "ScopeNextChannel", "ScopeAbortAll"} {
		if !strings.Contains(guardBody, required) {
			t.Fatalf("shouldRecordChannelFailure 缺少 %q 判定", required)
		}
	}
}
