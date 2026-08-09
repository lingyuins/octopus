package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
	"github.com/lingyuins/octopus/internal/op/pool"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

func setupPoolOAuthTest(t *testing.T) context.Context {
	t.Helper()

	crypto.Init("test-encryption-key")
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)

	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	// account_pools / pool_accounts 由迁移 040 创建（不在 AutoMigrate 主列表），
	// 而迁移注册表在进程内首次执行后被清空（AfterAutoMigrate 置 nil），
	// 多个测试各自 InitDB 时后续测试会漏建表，这里幂等兜底。
	if err := db.GetDB().AutoMigrate(&model.AccountPool{}, &model.PoolAccount{}); err != nil {
		t.Fatalf("auto migrate pool tables: %v", err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return context.Background()
}

func makeJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	enc := func(b []byte) string {
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return enc(header) + "." + enc(payload) + ".sig"
}

func TestDecodeJWTClaim_ExtractsSub(t *testing.T) {
	token := makeJWT(t, map[string]interface{}{"sub": "user-abc-123", "iss": "https://claude.ai"})
	if got := decodeJWTClaim(token, "sub"); got != "user-abc-123" {
		t.Fatalf("decodeJWTClaim(sub) = %q, want user-abc-123", got)
	}
	if got := decodeJWTClaim("not-a-jwt", "sub"); got != "" {
		t.Fatalf("decodeJWTClaim(garbage) = %q, want empty", got)
	}
	if got := decodeJWTClaim("", "sub"); got != "" {
		t.Fatalf("decodeJWTClaim(empty) = %q, want empty", got)
	}
}

func TestOAuthAccountKey(t *testing.T) {
	idToken := makeJWT(t, map[string]interface{}{"sub": "user-xyz"})
	accessToken := makeJWT(t, map[string]interface{}{"sub": "user-anthropic"})

	// openai：使用 account_id。
	if got := oauthAccountKey(model.PoolPlatformOpenAI, model.PoolCredential{AccountID: "acct-1"}); got != "acct-1" {
		t.Fatalf("openai key = %q, want acct-1", got)
	}
	// anthropic：优先 id_token 的 sub。
	if got := oauthAccountKey(model.PoolPlatformAnthropic, model.PoolCredential{IDToken: idToken, AccessToken: accessToken}); got != "user-xyz" {
		t.Fatalf("anthropic key = %q, want user-xyz", got)
	}
	// anthropic：无 id_token 时回退 access_token 的 sub。
	if got := oauthAccountKey(model.PoolPlatformAnthropic, model.PoolCredential{AccessToken: accessToken}); got != "user-anthropic" {
		t.Fatalf("anthropic fallback key = %q, want user-anthropic", got)
	}
	// gemini/grok：id_token 的 sub。
	if got := oauthAccountKey(model.PoolPlatformGrok, model.PoolCredential{IDToken: idToken}); got != "user-xyz" {
		t.Fatalf("grok key = %q, want user-xyz", got)
	}
	// 无稳定标识：返回空，跳过去重。
	if got := oauthAccountKey(model.PoolPlatformAnthropic, model.PoolCredential{AccessToken: "opaque-token"}); got != "" {
		t.Fatalf("opaque key = %q, want empty", got)
	}
}

func TestFindExistingOAuthAccount_DedupesByFingerprint(t *testing.T) {
	setupPoolOAuthTest(t)

	poolObj := &model.AccountPool{Name: "claude-pool"}
	if err := pool.CreatePool(poolObj); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	credJSON := `{"access_token":"tok-1","refresh_token":"ref-1","id_token":"` +
		makeJWT(t, map[string]interface{}{"sub": "user-same"}) + `"}`
	acct := &model.PoolAccount{
		PoolID:      poolObj.ID,
		Name:        "anthropic-oauth-1",
		Platform:    model.PoolPlatformAnthropic,
		Type:        model.PoolTypeOAuth,
		Credentials: pool.EncryptCredentials(credJSON),
		Status:      "active",
		Schedulable: true,
	}
	if err := pool.CreateAccount(acct); err != nil {
		t.Fatalf("create account: %v", err)
	}

	// 同一账号再次授权（新 token、相同 sub）→ 命中已有账号。
	sameCred := model.ParsePoolCredential(credJSON)
	got, err := findExistingOAuthAccount(poolObj.ID, model.PoolPlatformAnthropic, sameCred)
	if err != nil {
		t.Fatalf("find existing: %v", err)
	}
	if got != acct.ID {
		t.Fatalf("found account = %d, want %d", got, acct.ID)
	}

	// 不同账号（不同 sub）→ 不命中。
	otherCred := model.ParsePoolCredential(`{"access_token":"tok-2","id_token":"` +
		makeJWT(t, map[string]interface{}{"sub": "user-other"}) + `"}`)
	got, err = findExistingOAuthAccount(poolObj.ID, model.PoolPlatformAnthropic, otherCred)
	if err != nil {
		t.Fatalf("find other: %v", err)
	}
	if got != 0 {
		t.Fatalf("found account = %d, want 0", got)
	}

	// 不同平台（openai 但同 sub）→ 不命中。
	openaiCred := model.ParsePoolCredential(`{"access_token":"tok-3","account_id":"user-same"}`)
	got, err = findExistingOAuthAccount(poolObj.ID, model.PoolPlatformOpenAI, openaiCred)
	if err != nil {
		t.Fatalf("find openai: %v", err)
	}
	if got != 0 {
		t.Fatalf("found account = %d, want 0", got)
	}
}

func TestRefreshExistingOAuthAccount_ResetsState(t *testing.T) {
	setupPoolOAuthTest(t)

	poolObj := &model.AccountPool{Name: "claude-pool"}
	if err := pool.CreatePool(poolObj); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	// 既有账号：已过期、被暂停、有报错与刷新失败退避。
	extra := `{"refresh_failure_count":3,"next_refresh_allowed_at":9999999999}`
	acct := &model.PoolAccount{
		PoolID:         poolObj.ID,
		Name:           "anthropic-oauth-1",
		Platform:       model.PoolPlatformAnthropic,
		Type:           model.PoolTypeOAuth,
		Credentials:    pool.EncryptCredentials(`{"access_token":"old","id_token":"` + makeJWT(t, map[string]interface{}{"sub": "user-1"}) + `"}`),
		Status:         "paused",
		Schedulable:    false,
		ErrorMessage:   "auth failed",
		TokenExpiresAt: 100,
		Extra:          extra,
	}
	if err := pool.CreateAccount(acct); err != nil {
		t.Fatalf("create account: %v", err)
	}

	newCredJSON := `{"access_token":"new-token","refresh_token":"new-refresh","id_token":"` +
		makeJWT(t, map[string]interface{}{"sub": "user-1"}) + `"}`
	expiresAt := int64(2000000000)
	if err := refreshExistingOAuthAccount(poolObj.ID, acct.ID, newCredJSON, expiresAt, nil); err != nil {
		t.Fatalf("refresh existing: %v", err)
	}

	got, err := pool.GetAccount(poolObj.ID, acct.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.TokenExpiresAt != expiresAt {
		t.Fatalf("token_expires_at = %d, want %d", got.TokenExpiresAt, expiresAt)
	}
	if got.Status != "active" || !got.Schedulable || got.ErrorMessage != "" {
		t.Fatalf("state not reset: status=%q schedulable=%v error=%q", got.Status, got.Schedulable, got.ErrorMessage)
	}
	if err := pool.DecryptAccountCredentials(got); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	cred := model.ParsePoolCredential(got.Credentials)
	if cred.AccessToken != "new-token" || cred.RefreshToken != "new-refresh" {
		t.Fatalf("credentials not updated: %+v", cred)
	}
	var extraAfter model.PoolAccountExtra
	if err := json.Unmarshal([]byte(got.Extra), &extraAfter); err != nil {
		t.Fatalf("parse extra: %v", err)
	}
	if extraAfter.RefreshFailureCount != 0 || extraAfter.NextRefreshAllowedAt != 0 {
		t.Fatalf("refresh backoff not cleared: %+v", extraAfter)
	}
}
