package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/helper"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op"
)

func setupAlertHandlerTest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	testName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", testName)
	if err := db.InitDB("sqlite", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := op.InitCache(); err != nil {
		t.Fatalf("init cache: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
}

func TestDeleteAlertRuleNotFoundReturns404(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/alert/rule/delete/404", nil)
	c.Params = gin.Params{{Key: "id", Value: "404"}}

	deleteAlertRule(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestCreateAlertRuleIgnoresReadonlyID(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/rule/create", strings.NewReader(`{
		"id":404,
		"name":"created",
		"enabled":true,
		"condition_type":"error_rate",
		"threshold":10,
		"cooldown_sec":300
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	createAlertRule(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Data model.AlertRule `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID == 404 {
		t.Fatalf("create response preserved client supplied id")
	}
}

func TestUpdateAlertRuleNotFoundReturns404(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/rule/update", strings.NewReader(`{
		"id":404,
		"name":"missing",
		"enabled":true,
		"condition_type":"error_rate",
		"threshold":10,
		"cooldown_sec":300
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	updateAlertRule(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestDeleteNotifChannelNotFoundReturns404(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/alert/notif/delete/404", nil)
	c.Params = gin.Params{{Key: "id", Value: "404"}}

	deleteNotifChannel(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestCreateNotifChannelIgnoresReadonlyID(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/notif/create", strings.NewReader(`{
		"id":404,
		"name":"created",
		"type":"webhook",
		"url":"https://example.com/hook"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	createNotifChannel(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Data model.AlertNotifChannel `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID == 404 {
		t.Fatalf("create response preserved client supplied id")
	}
}

func TestUpdateNotifChannelNotFoundReturns404(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/notif/update", strings.NewReader(`{
		"id":404,
		"name":"missing",
		"type":"webhook",
		"url":"https://example.com/hook"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	updateNotifChannel(c)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestNotifChannelReportsConfigError(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	// gotify channel with neither config nor url/secret fallbacks -> validation error
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/notif/test", strings.NewReader(`{
		"name":"bad-gotify",
		"type":"gotify"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	testNotifChannel(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "gotify") {
		t.Fatalf("expected error to mention gotify, got: %s", recorder.Body.String())
	}
}

func TestNotifChannelSucceedsAgainstWebhook(t *testing.T) {
	setupAlertHandlerTest(t)

	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 出站发送链路（op 层）不受 handler 的 SSRF 校验影响，直接验证发送成功。
	ch := &model.AlertNotifChannel{Name: "my-webhook", Type: "webhook", URL: srv.URL}
	if err := helper.TestNotification(ch); err != nil {
		t.Fatalf("TestNotification failed: %v", err)
	}
	if received == nil {
		t.Fatalf("webhook server did not receive a request")
	}
	if received["state"] != "test" {
		t.Fatalf("expected state=test in webhook payload, got: %v", received["state"])
	}
}

// TestNotifChannelRejectsInternalWebhookURL 验证 handler 层拒绝内网 webhook URL
// （SSRF 防护）：loopback 地址的 test 请求应返回 400。
func TestNotifChannelRejectsInternalWebhookURL(t *testing.T) {
	setupAlertHandlerTest(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"name":"internal-webhook","type":"webhook","url":"http://127.0.0.1:9999/hook"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/alert/notif/test", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	testNotifChannel(c)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "unsafe") {
		t.Fatalf("expected unsafe url error, got: %s", recorder.Body.String())
	}
}
