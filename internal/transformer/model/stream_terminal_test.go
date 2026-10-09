package model

import (
	"strings"
	"testing"

	"github.com/lingyuins/octopus/internal/transformer"
)

func TestStreamFinishedIsNotSerialized(t *testing.T) {
	body, err := transformer.Marshal(&InternalLLMResponse{ID: "test", Object: "chat.completion.chunk", StreamFinished: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "StreamFinished") || strings.Contains(string(body), "stream_finished") {
		t.Fatalf("internal terminal flag leaked into wire response: %s", body)
	}
}
