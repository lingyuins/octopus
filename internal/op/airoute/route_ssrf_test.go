package airoute

import (
	"context"
	"strings"
	"testing"
)

// issue #221 回归测试：AI Route 的 BaseURL 指向内网/元数据地址时，
// 出站请求必须在拨号前被 SSRF 校验拒绝。
func TestGenerateAIRoutesRejectsDisallowedBaseURL(t *testing.T) {
	cases := []string{
		"http://169.254.169.254",
		"http://127.0.0.1:8080",
		"http://localhost:1234",
		"http://10.0.0.5/v1",
	}
	for _, baseURL := range cases {
		_, err := generateAIRoutesForBucketWithService(
			context.Background(),
			aiRouteService{Name: "test", BaseURL: baseURL, APIKey: "sk-test", Model: "test-model"},
			aiRoutePromptBucket{PromptEndpointType: "openai"},
			"test-group",
			0, 1, nil,
		)
		if err == nil {
			t.Fatalf("baseURL %s: expected ssrf rejection, got nil error", baseURL)
		}
		if !strings.Contains(err.Error(), "不允许访问") {
			t.Fatalf("baseURL %s: expected ssrf rejection error, got: %v", baseURL, err)
		}
	}
}
