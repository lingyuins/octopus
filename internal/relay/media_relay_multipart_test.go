package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExtractModelFromMultipartHandlesMissingBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader(""))
	ctx.Request.Header.Set("Content-Type", "multipart/form-data")

	_, _, _, err := extractModelFromMultipart(ctx)
	if err == nil {
		t.Fatal("extractModelFromMultipart() expected error for missing boundary, got nil")
	}
}

func TestExtractModelFromMultipartHandlesMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader("not a valid multipart body"))
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=abc123")

	_, _, _, err := extractModelFromMultipart(ctx)
	if err == nil {
		t.Fatal("extractModelFromMultipart() expected error for malformed body, got nil")
	}
}
