package relay

import (
    "testing"

    dbmodel "github.com/lingyuins/octopus/internal/model"
    tmodel "github.com/lingyuins/octopus/internal/transformer/model"
    "github.com/samber/lo"
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


func TestRewriteConversationRequestByProvider_SkipsAllEndpoint(t *testing.T) {
	req := &tmodel.InternalLLMRequest{Messages: []tmodel.Message{{Role: "assistant", Reasoning: lo.ToPtr("r")}}}
	got := rewriteConversationRequestByProvider(dbmodel.Group{EndpointType: dbmodel.EndpointTypeAll, EndpointProvider: "deepseek"}, req)
	if got.Messages[0].Reasoning == nil {
		t.Fatal("all endpoint should not rewrite reasoning fields")
	}
}

func TestRewriteConversationRequestByProvider_DeepSeekClearsReasoningAlias(t *testing.T) {
	req := &tmodel.InternalLLMRequest{Messages: []tmodel.Message{{Role: "assistant", Reasoning: lo.ToPtr("r"), ReasoningContent: lo.ToPtr("c")}}}
	got := rewriteConversationRequestByProvider(dbmodel.Group{EndpointType: dbmodel.EndpointTypeChat, EndpointProvider: "deepseek"}, req)
	if got.Messages[0].Reasoning != nil {
		t.Fatal("deepseek provider should clear reasoning alias field")
	}
	if got.Messages[0].ReasoningContent == nil {
		t.Fatal("deepseek provider should preserve reasoning_content")
	}
}
