package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/server/auth"
	"github.com/lingyuins/octopus/internal/server/middleware"
	"github.com/lingyuins/octopus/internal/server/resp"
	"github.com/lingyuins/octopus/internal/server/router"
)

func init() {
	router.NewGroupRouter("/api/v1/cli-export").
		Use(middleware.Auth()).
		Use(middleware.RequirePermission(auth.PermAPIKeysRead)).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/generate", http.MethodPost).
				Handle(generateCLIExport),
		)
}

func generateCLIExport(c *gin.Context) {
	var req model.CLIExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, http.StatusBadRequest, resp.ErrInvalidJSON)
		return
	}

	baseURL := strings.TrimRight(req.BaseURL, "/")

	var result model.CLIExportResult
	result.Tool = req.Tool

	switch req.Tool {
	case "claude_code":
		result = buildClaudeCodeExport(baseURL, req.APIKey)
	case "codex":
		result = buildCodexExport(baseURL, req.APIKey)
	case "gemini_cli":
		result = buildGeminiCLIExport(baseURL, req.APIKey)
	case "cherry_studio":
		result = buildCherryStudioExport(baseURL, req.APIKey, req.APIType)
	case "kilo_code":
		result = buildKiloCodeExport(baseURL, req.APIKey)
	default:
		resp.Error(c, http.StatusBadRequest, fmt.Sprintf("unsupported tool: %s", req.Tool))
		return
	}

	resp.Success(c, result)
}

func buildClaudeCodeExport(baseURL, apiKey string) model.CLIExportResult {
	envLines := fmt.Sprintf(
		"ANTHROPIC_BASE_URL=%s\nANTHROPIC_API_KEY=%s",
		baseURL, apiKey,
	)
	return model.CLIExportResult{
		Tool:        "claude_code",
		Format:      "env",
		Content:     envLines,
		Filename:    ".env",
		Description: "Set these environment variables before running Claude Code CLI",
	}
}

func buildCodexExport(baseURL, apiKey string) model.CLIExportResult {
	envLines := fmt.Sprintf(
		"OPENAI_BASE_URL=%s/v1\nOPENAI_API_KEY=%s",
		baseURL, apiKey,
	)
	return model.CLIExportResult{
		Tool:        "codex",
		Format:      "env",
		Content:     envLines,
		Filename:    ".env",
		Description: "Set these environment variables before running Codex CLI",
	}
}

func buildGeminiCLIExport(baseURL, apiKey string) model.CLIExportResult {
	envLines := fmt.Sprintf(
		"GEMINI_API_KEY=%s\nGEMINI_BASE_URL=%s",
		apiKey, baseURL,
	)
	return model.CLIExportResult{
		Tool:        "gemini_cli",
		Format:      "env",
		Content:     envLines,
		Filename:    ".env",
		Description: "Set these environment variables before running Gemini CLI",
	}
}

func buildCherryStudioExport(baseURL, apiKey, apiType string) model.CLIExportResult {
	provider := map[string]interface{}{
		"id":       "custom",
		"name":     "Custom Provider",
		"apiKey":   apiKey,
		"apiHost":  baseURL,
		"isActive": true,
	}
	if apiType != "" {
		provider["type"] = apiType
	}

	b, _ := json.MarshalIndent(provider, "", "  ")
	return model.CLIExportResult{
		Tool:        "cherry_studio",
		Format:      "json",
		Content:     string(b),
		Filename:    "cherry-studio-provider.json",
		Description: "Import this JSON as a custom provider in Cherry Studio settings",
	}
}

func buildKiloCodeExport(baseURL, apiKey string) model.CLIExportResult {
	config := map[string]interface{}{
		"apiProvider": "openai-compatible",
		"openAiCompatible": map[string]string{
			"baseUrl": baseURL + "/v1",
			"apiKey":  apiKey,
		},
	}

	b, _ := json.MarshalIndent(config, "", "  ")
	return model.CLIExportResult{
		Tool:        "kilo_code",
		Format:      "json",
		Content:     string(b),
		Filename:    "kilo-code-settings.json",
		Description: "Paste this into Kilo Code's provider settings",
	}
}
