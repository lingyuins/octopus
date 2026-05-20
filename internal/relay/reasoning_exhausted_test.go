package relay

import (
    "testing"

    tmodel "github.com/lingyuins/octopus/internal/transformer/model"
)

func TestIsReasoningExhaustedResponse_EmptyContentWithReasoningTokens(t *testing.T) {
    empty := ""
    resp := &tmodel.InternalLLMResponse{
        Choices: []tmodel.Choice{{
            Index: 0,
            Message: &tmodel.Message{
                Role: "assistant",
                Content: tmodel.MessageContent{Content: &empty},
            },
        }},
        Usage: &tmodel.Usage{
            CompletionTokens: 50,
            CompletionTokensDetails: &tmodel.CompletionTokensDetails{
                ReasoningTokens: 49,
            },
        },
    }

    if !isReasoningExhaustedResponse(resp) {
        t.Fatal("expected reasoning exhaustion to be detected")
    }
}

func TestIsReasoningExhaustedResponse_WithVisibleContent(t *testing.T) {
    content := "Hello!"
    resp := &tmodel.InternalLLMResponse{
        Choices: []tmodel.Choice{{
            Index: 0,
            Message: &tmodel.Message{
                Role: "assistant",
                Content: tmodel.MessageContent{Content: &content},
            },
        }},
        Usage: &tmodel.Usage{
            CompletionTokens: 50,
            CompletionTokensDetails: &tmodel.CompletionTokensDetails{
                ReasoningTokens: 49,
            },
        },
    }

    if isReasoningExhaustedResponse(resp) {
        t.Fatal("expected visible content to skip reasoning exhaustion marker")
    }
}
