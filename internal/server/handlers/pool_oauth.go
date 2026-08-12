package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/pool"
	"github.com/lingyuins/octopus/internal/pkg/geminicli"
	"github.com/lingyuins/octopus/internal/pkg/oauth"
	"github.com/lingyuins/octopus/internal/pkg/openai"
	"github.com/lingyuins/octopus/internal/pkg/xai"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/router"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// OAuth 回调端点不挂 Auth middleware：回调阶段用 state/session 校验。
// initiate 阶段需要登录态：前端先通过管理 API（带 JWT）发起，这里挂 Auth 保护。
func init() {
	router.NewGroupRouter("/api/v1/pool/oauth").
		AddRoute(
			router.NewRoute("/initiate", http.MethodGet).
				Use(middleware.Auth()).
				Use(middleware.RequirePermission(auth.PermChannelsWrite)).
				Handle(oauthInitiate),
		).
		AddRoute(
			router.NewRoute("/callback", http.MethodGet).Handle(oauthCallback),
		)
}

// 全局 SessionStore（4 平台共用，按 sessionID 区分）。
var (
	anthropicSessions = oauth.NewSessionStore()
	openaiSessions    = openai.NewSessionStore()
	geminiSessions    = geminicli.NewSessionStore()
	grokSessions      = xai.NewSessionStore()
)

// externalRedirectURL 返回 OAuth 回调地址：{ExternalURL}/api/v1/pool/oauth/callback。
func externalRedirectURL() (string, error) {
	base := strings.TrimRight(strings.TrimSpace(conf.AppConfig.Server.ExternalURL), "/")
	if base == "" {
		host := conf.AppConfig.Server.Host
		port := conf.AppConfig.Server.Port
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		}
		base = fmt.Sprintf("http://%s:%d", host, port)
	}
	return base + "/api/v1/pool/oauth/callback", nil
}

func oauthInitiate(c *gin.Context) {
	platform := strings.TrimSpace(c.Query("platform"))
	poolIDStr := c.Query("pool_id")
	poolID, err := strconv.Atoi(poolIDStr)
	if err != nil || poolID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pool_id"})
		return
	}

	redirectURI, err := externalRedirectURL()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch platform {
	case model.PoolPlatformAnthropic:
		state, err := oauth.GenerateState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate state failed"})
			return
		}
		verifier, err := oauth.GenerateCodeVerifier()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate verifier failed"})
			return
		}
		challenge := oauth.GenerateCodeChallenge(verifier)
		sessionID, err := oauth.GenerateSessionID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate session failed"})
			return
		}
		anthropicSessions.Set(sessionID, &oauth.OAuthSession{
			State:        state,
			CodeVerifier: verifier,
			Scope:        oauth.ScopeOAuth,
			PoolID:       poolID,
			RedirectURI:  redirectURI,
			CreatedAt:    time.Now(),
		})
		authURL := oauth.BuildAuthorizationURL(state, challenge, oauth.ScopeOAuth, redirectURI)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "session_id": sessionID})

	case model.PoolPlatformOpenAI:
		state, err := openai.GenerateState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate state failed"})
			return
		}
		verifier, err := openai.GenerateCodeVerifier()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate verifier failed"})
			return
		}
		challenge := openai.GenerateCodeChallenge(verifier)
		sessionID, err := openai.GenerateSessionID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate session failed"})
			return
		}
		openaiSessions.Set(sessionID, &openai.OAuthSession{
			State:        state,
			CodeVerifier: verifier,
			RedirectURI:  redirectURI,
			PoolID:       poolID,
			CreatedAt:    time.Now(),
		})
		authURL := openai.BuildAuthorizationURL(state, challenge, redirectURI)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "session_id": sessionID})

	case model.PoolPlatformGemini:
		state, err := geminicli.GenerateState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate state failed"})
			return
		}
		verifier, err := geminicli.GenerateCodeVerifier()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate verifier failed"})
			return
		}
		challenge := geminicli.GenerateCodeChallenge(verifier)
		sessionID, err := geminicli.GenerateSessionID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate session failed"})
			return
		}
		geminiSessions.Set(sessionID, &geminicli.OAuthSession{
			State:        state,
			CodeVerifier: verifier,
			RedirectURI:  redirectURI,
			PoolID:       poolID,
			CreatedAt:    time.Now(),
		})
		authURL, err := geminicli.BuildAuthorizationURL(geminicli.OAuthConfig{}, state, challenge, redirectURI)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "session_id": sessionID})

	case model.PoolPlatformGrok:
		state, err := xai.GenerateState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate state failed"})
			return
		}
		verifier, err := xai.GenerateCodeVerifier()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate verifier failed"})
			return
		}
		challenge := xai.GenerateCodeChallenge(verifier)
		nonce, err := xai.GenerateNonce()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate nonce failed"})
			return
		}
		sessionID, err := xai.GenerateSessionID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "generate session failed"})
			return
		}
		grokSessions.Set(sessionID, &xai.OAuthSession{
			State:         state,
			CodeVerifier:  verifier,
			CodeChallenge: challenge,
			RedirectURI:   redirectURI,
			PoolID:        poolID,
			CreatedAt:     time.Now(),
		})
		authURL, err := xai.BuildAuthorizationURL(state, challenge, redirectURI, nonce)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "session_id": sessionID})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform for oauth: " + platform})
	}
}

func oauthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	sessionID := c.Query("session_id")
	platform := c.Query("platform")

	if code == "" || state == "" {
		oauthRedirectResult(c, false, 0, "missing code or state")
		return
	}

	var poolID int
	var credJSON string
	var expiresAt int64
	var err error

	switch platform {
	case model.PoolPlatformAnthropic:
		poolID, credJSON, expiresAt, err = handleAnthropicCallback(c.Request.Context(), sessionID, state, code)
	case model.PoolPlatformOpenAI:
		poolID, credJSON, expiresAt, err = handleOpenAICallback(c.Request.Context(), sessionID, state, code)
	case model.PoolPlatformGemini:
		poolID, credJSON, expiresAt, err = handleGeminiCallback(c.Request.Context(), sessionID, state, code)
	case model.PoolPlatformGrok:
		poolID, credJSON, expiresAt, err = handleGrokCallback(c.Request.Context(), sessionID, state, code)
	default:
		oauthRedirectResult(c, false, 0, "unsupported platform: "+platform)
		return
	}

	if err != nil {
		log.Warnf("oauth callback %s failed: %v", platform, err)
		oauthRedirectResult(c, false, 0, err.Error())
		return
	}

	// 去重：同一平台账号指纹（openai account_id / JWT sub）已存在时复用并更新令牌，
	// 避免同一账号重复授权在号池里生成重复账号。
	cred := model.ParsePoolCredential(credJSON)
	existingID, err := findExistingOAuthAccount(poolID, platform, cred)
	if err != nil {
		oauthRedirectResult(c, false, poolID, "dedupe lookup failed: "+err.Error())
		return
	}
	// gemini OAuth 必须记录 code_assist 标记与 project id，否则出站会退化成
	// 「把 OAuth token 当官方 API key 的 ?key= 参数」，必然 401/403。
	var geminiExtra *model.PoolAccountExtra
	if platform == model.PoolPlatformGemini {
		e := discoverGeminiExtra(c.Request.Context(), cred.AccessToken)
		geminiExtra = &e
	}

	if existingID > 0 {
		if err := refreshExistingOAuthAccount(poolID, existingID, credJSON, expiresAt, geminiExtra); err != nil {
			oauthRedirectResult(c, false, poolID, "update account failed: "+err.Error())
			return
		}
		oauthRedirectResult(c, true, poolID, strconv.Itoa(existingID))
		return
	}

	// 新建号池账号。
	acct := model.PoolAccount{
		PoolID:         poolID,
		Name:           fmt.Sprintf("%s-oauth-%d", platform, time.Now().Unix()),
		Platform:       platform,
		Type:           model.PoolTypeOAuth,
		Credentials:    pool.EncryptCredentials(credJSON),
		Status:         "active",
		Schedulable:    true,
		TokenExpiresAt: expiresAt,
	}
	if geminiExtra != nil {
		acct.SetExtra(*geminiExtra)
	}
	if err := pool.CreateAccount(&acct); err != nil {
		oauthRedirectResult(c, false, poolID, "create account failed: "+err.Error())
		return
	}
	oauthRedirectResult(c, true, poolID, strconv.Itoa(acct.ID))
}

// oauthAccountKey 计算 OAuth 账号的稳定指纹，用于号池内去重。
// openai 使用 chatgpt_account_id；anthropic/gemini/grok 优先取 id_token 的 sub，
// 其次 access_token 的 sub（anthropic 的 access_token 为 JWT）。
// 无法确定标识时返回空串，调用方据此跳过去重（避免误合并）。
func oauthAccountKey(platform string, cred model.PoolCredential) string {
	switch platform {
	case model.PoolPlatformOpenAI:
		return cred.AccountID
	case model.PoolPlatformAnthropic, model.PoolPlatformGemini, model.PoolPlatformGrok:
		for _, token := range []string{cred.IDToken, cred.AccessToken} {
			if sub := decodeJWTClaim(token, "sub"); sub != "" {
				return sub
			}
		}
	}
	return ""
}

// findExistingOAuthAccount 在同 pool 内查找与本次授权同平台、同账号指纹的 oauth 账号。
// 返回账号 ID；无指纹或未找到返回 0。
func findExistingOAuthAccount(poolID int, platform string, cred model.PoolCredential) (int, error) {
	key := oauthAccountKey(platform, cred)
	if key == "" {
		return 0, nil
	}
	accounts, err := pool.ListAccounts(poolID)
	if err != nil {
		return 0, err
	}
	for i := range accounts {
		acct := accounts[i]
		if acct.Platform != platform || acct.Type != model.PoolTypeOAuth {
			continue
		}
		if err := pool.DecryptAccountCredentials(&acct); err != nil {
			continue
		}
		existingCred := model.ParsePoolCredential(acct.Credentials)
		if oauthAccountKey(platform, existingCred) == key {
			return acct.ID, nil
		}
	}
	return 0, nil
}

// refreshExistingOAuthAccount 复用已有账号：更新凭据/过期时间并复位状态，
// 同时清除刷新失败退避（授权成功视为账号恢复）。
//
// platformExtra 非空时把平台字段（如 gemini 的 code_assist project）合并进 Extra，
// 其余字段保持原样。
func refreshExistingOAuthAccount(poolID, accountID int, credJSON string, expiresAt int64, platformExtra *model.PoolAccountExtra) error {
	updates := map[string]interface{}{
		"credentials":      pool.EncryptCredentials(credJSON),
		"token_expires_at": expiresAt,
		"status":           "active",
		"schedulable":      true,
		"error_message":    "",
	}
	if acct, err := pool.GetAccount(poolID, accountID); err == nil {
		var extra model.PoolAccountExtra
		if json.Unmarshal([]byte(acct.Extra), &extra) == nil {
			changed := false
			if extra.RefreshFailureCount != 0 || extra.NextRefreshAllowedAt != 0 {
				extra.RefreshFailureCount = 0
				extra.NextRefreshAllowedAt = 0
				changed = true
			}
			if platformExtra != nil {
				if platformExtra.OAuthType != "" && extra.OAuthType != platformExtra.OAuthType {
					extra.OAuthType = platformExtra.OAuthType
					changed = true
				}
				// project 探测失败时保留原值，避免把已可用的 project 抹掉。
				if platformExtra.ProjectID != "" && extra.ProjectID != platformExtra.ProjectID {
					extra.ProjectID = platformExtra.ProjectID
					changed = true
				}
			}
			if changed {
				if b, err := json.Marshal(extra); err == nil {
					updates["extra"] = string(b)
				}
			}
		}
	}
	return pool.UpdateAccount(poolID, accountID, updates)
}

// decodeJWTClaim 从 JWT payload 中提取指定 claim（仅支持字符串值）。
func decodeJWTClaim(token, claim string) string {
	if token == "" {
		return ""
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if v, ok := claims[claim].(string); ok {
		return v
	}
	return ""
}

// handleAnthropicCallback 处理 Anthropic OAuth 回调：校验 session，code exchange。
func handleAnthropicCallback(ctx context.Context, sessionID, state, code string) (int, string, int64, error) {
	session, ok := anthropicSessions.Get(sessionID)
	if !ok {
		return 0, "", 0, fmt.Errorf("session expired")
	}
	defer anthropicSessions.Delete(sessionID)
	if session.State != state {
		return 0, "", 0, fmt.Errorf("state mismatch")
	}
	tok, err := exchangeCode(ctx, "https://console.anthropic.com/v1/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {oauth.ClientID},
		"code":          {code},
		"code_verifier": {session.CodeVerifier},
		"redirect_uri":  {session.RedirectURI},
	})
	if err != nil {
		return 0, "", 0, err
	}
	cred := model.PoolCredential{
		Type:         model.PoolTypeOAuth,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
	}
	credBytes, _ := json.Marshal(cred)
	expiresAt := time.Now().Unix() + tok.ExpiresIn
	return session.PoolID, string(credBytes), expiresAt, nil
}

func handleOpenAICallback(ctx context.Context, sessionID, state, code string) (int, string, int64, error) {
	session, ok := openaiSessions.Get(sessionID)
	if !ok {
		return 0, "", 0, fmt.Errorf("session expired")
	}
	defer openaiSessions.Delete(sessionID)
	if session.State != state {
		return 0, "", 0, fmt.Errorf("state mismatch")
	}
	tok, err := exchangeCode(ctx, "https://auth.openai.com/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {openai.ClientID},
		"code":          {code},
		"code_verifier": {session.CodeVerifier},
		"redirect_uri":  {session.RedirectURI},
	})
	if err != nil {
		return 0, "", 0, err
	}
	// 从 id_token 解析 chatgpt_account_id。
	accountID := decodeOpenAIAccountID(tok.IDToken)
	cred := model.PoolCredential{
		Type:         model.PoolTypeOAuth,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		IDToken:      tok.IDToken,
		AccountID:    accountID,
	}
	credBytes, _ := json.Marshal(cred)
	expiresAt := time.Now().Unix() + tok.ExpiresIn
	return session.PoolID, string(credBytes), expiresAt, nil
}

func handleGeminiCallback(ctx context.Context, sessionID, state, code string) (int, string, int64, error) {
	session, ok := geminiSessions.Get(sessionID)
	if !ok {
		return 0, "", 0, fmt.Errorf("session expired")
	}
	defer geminiSessions.Delete(sessionID)
	if session.State != state {
		return 0, "", 0, fmt.Errorf("state mismatch")
	}
	effective, err := geminicli.EffectiveOAuthConfig(geminicli.OAuthConfig{})
	if err != nil {
		return 0, "", 0, err
	}
	tok, err := exchangeCode(ctx, "https://oauth2.googleapis.com/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {effective.ClientID},
		"client_secret": {effective.ClientSecret},
		"code":          {code},
		"code_verifier": {session.CodeVerifier},
		"redirect_uri":  {session.RedirectURI},
	})
	if err != nil {
		return 0, "", 0, err
	}
	cred := model.PoolCredential{
		Type:         model.PoolTypeOAuth,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		IDToken:      tok.IDToken,
	}
	credBytes, _ := json.Marshal(cred)
	expiresAt := time.Now().Unix() + tok.ExpiresIn
	return session.PoolID, string(credBytes), expiresAt, nil
}

// discoverGeminiExtra 探测 gemini OAuth 账号的 Cloud Code Assist project，
// 组装为账号 Extra。出站请求体需要 project id（见 transformer/outbound/gemini）。
//
// 探测失败不阻断授权：project 为空时出站仍按无 project 方式尝试，
// 用户也可后续在账号详情里手工补填。
func discoverGeminiExtra(ctx context.Context, accessToken string) model.PoolAccountExtra {
	projectID, err := geminicli.DiscoverProject(ctx, nil, accessToken, geminicli.CodeAssistEndpoint)
	if err != nil {
		log.Warnf("gemini oauth: discover code assist project failed: %v", err)
	}
	return model.PoolAccountExtra{
		ProjectID: projectID,
		OAuthType: model.OAuthTypeCodeAssist,
	}
}

func handleGrokCallback(ctx context.Context, sessionID, state, code string) (int, string, int64, error) {
	session, ok := grokSessions.Get(sessionID)
	if !ok {
		return 0, "", 0, fmt.Errorf("session expired")
	}
	defer grokSessions.Delete(sessionID)
	if session.State != state {
		return 0, "", 0, fmt.Errorf("state mismatch")
	}
	tok, err := exchangeCode(ctx, "https://auth.x.ai/oauth2/token", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {xai.DefaultClientID},
		"code":          {code},
		"code_verifier": {session.CodeVerifier},
		"redirect_uri":  {session.RedirectURI},
	})
	if err != nil {
		return 0, "", 0, err
	}
	cred := model.PoolCredential{
		Type:         model.PoolTypeOAuth,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		IDToken:      tok.IDToken,
	}
	credBytes, _ := json.Marshal(cred)
	expiresAt := time.Now().Unix() + tok.ExpiresIn
	return session.PoolID, string(credBytes), expiresAt, nil
}

// exchangeCode 通用 OAuth code exchange。
type tokenExchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
}

func exchangeCode(ctx context.Context, tokenURL string, form url.Values) (*tokenExchangeResponse, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var tok tokenExchangeResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	return &tok, nil
}

// decodeOpenAIAccountID 从 OpenAI id_token JWT payload 解析 chatgpt_account_id。
func decodeOpenAIAccountID(idToken string) string {
	if idToken == "" {
		return ""
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		OpenAIAuth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.OpenAIAuth.ChatGPTAccountID
}

// base64URLDecode 解码 base64url（补齐 padding）。
func base64URLDecode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	for len(s)%4 != 0 {
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}

// oauthRedirectResult 302 回前端 /pool?oauth=success|error&pool_id=N&msg=...。
func oauthRedirectResult(c *gin.Context, success bool, poolID int, msg string) {
	base := strings.TrimRight(strings.TrimSpace(conf.AppConfig.Server.ExternalURL), "/")
	if base == "" {
		// 回退到请求 Host。
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	status := "success"
	if !success {
		status = "error"
	}
	location := fmt.Sprintf("%s/pool?oauth=%s&pool_id=%d&msg=%s", base, status, poolID, url.QueryEscape(msg))
	c.Redirect(http.StatusFound, location)
}
