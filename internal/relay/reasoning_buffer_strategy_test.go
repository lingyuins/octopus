package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	dbmodel "github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	tmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func TestGetReasoningBufferStrategy(t *testing.T) {
	cache := setting.GetCache()
	key := dbmodel.SettingKeyReasoningBufferStrategy
	original, existed := cache.Get(key)
	t.Cleanup(func() {
		if existed {
			cache.Set(key, original)
		} else {
			cache.Del(key)
		}
	})

	tests := []struct {
		name   string
		global *string
		group  *dbmodel.Group
		want   string
	}{
		{name: "missing setting", want: "immediate"},
		{name: "empty group inherits default", group: &dbmodel.Group{}, want: "immediate"},
		{name: "empty setting", global: strategyValue(""), want: "immediate"},
		{name: "invalid setting", global: strategyValue("invalid"), want: "immediate"},
		{name: "global buffer preserved", global: strategyValue("buffer"), want: "buffer"},
		{name: "global immediate", global: strategyValue("immediate"), want: "immediate"},
		{name: "group inherits global buffer", global: strategyValue("buffer"), group: &dbmodel.Group{}, want: "buffer"},
		{name: "group buffer overrides global", global: strategyValue("immediate"), group: &dbmodel.Group{ReasoningBufferStrategy: "buffer"}, want: "buffer"},
		{name: "group immediate overrides global", global: strategyValue("buffer"), group: &dbmodel.Group{ReasoningBufferStrategy: "immediate"}, want: "immediate"},
		{name: "invalid group inherits global", global: strategyValue("buffer"), group: &dbmodel.Group{ReasoningBufferStrategy: "invalid"}, want: "buffer"},
		{name: "invalid group falls back to default", group: &dbmodel.Group{ReasoningBufferStrategy: "invalid"}, want: "immediate"},
		{name: "whitespace normalized", global: strategyValue(" buffer "), want: "buffer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.Del(key)
			if tt.global != nil {
				cache.Set(key, *tt.global)
			}
			if got := getReasoningBufferStrategy(tt.group); got != tt.want {
				t.Fatalf("getReasoningBufferStrategy() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strategyValue(value string) *string {
	return &value
}

type reasoningInbound struct{ stubInboundAdapter }

func (reasoningInbound) TransformStream(_ context.Context, stream *tmodel.InternalLLMResponse) ([]byte, error) {
	for _, choice := range stream.Choices {
		if choice.Delta != nil && choice.Delta.ReasoningContent != nil {
			return []byte("data: " + *choice.Delta.ReasoningContent + "\n\n"), nil
		}
	}
	return nil, nil
}

type reasoningFlushRecorder struct {
	*httptest.ResponseRecorder
	flushed chan struct{}
}

func (w *reasoningFlushRecorder) Flush() {
	w.ResponseRecorder.Flush()
	select {
	case w.flushed <- struct{}{}:
	default:
	}
}

func TestHandleStreamResponseDefaultSendsReasoningBeforeCompletion(t *testing.T) {
	cache := setting.GetCache()
	key := dbmodel.SettingKeyReasoningBufferStrategy
	original, existed := cache.Get(key)
	cache.Del(key)
	t.Cleanup(func() {
		if existed {
			cache.Set(key, original)
		} else {
			cache.Del(key)
		}
	})

	clientCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ra, recorder := newStreamTestAttempt(t, clientCtx)
	ra.outAdapter = reasoningOnlyOutbound{}
	ra.inAdapter = reasoningInbound{}
	writer := &reasoningFlushRecorder{ResponseRecorder: recorder, flushed: make(chan struct{}, 1)}
	c, _ := gin.CreateTestContext(writer)
	c.Request = ra.c.Request
	ra.c = c
	attachStreamSession(t, ra)

	reader, upstream := io.Pipe()
	t.Cleanup(func() { _ = upstream.Close() })
	t.Cleanup(func() { _ = reader.Close() })
	done := make(chan error, 1)
	go func() {
		done <- ra.handleStreamResponse(context.Background(), &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       reader,
		})
	}()
	go func() { _, _ = upstream.Write([]byte("data: thinking\n\n")) }()

	select {
	case <-writer.flushed:
	case <-time.After(3 * time.Second):
		t.Error("reasoning chunk was buffered until completion instead of flushed immediately")
	}
	if err := upstream.Close(); err != nil {
		t.Fatalf("close upstream: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleStreamResponse() = %v, want nil without empty-output retry", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handleStreamResponse() blocked after upstream EOF")
	}
	if !recorder.Flushed || !strings.Contains(recorder.Body.String(), "data: thinking") {
		t.Fatalf("reasoning-only stream was not sent: %q", recorder.Body.String())
	}
}
