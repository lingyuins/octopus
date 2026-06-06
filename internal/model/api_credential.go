package model

import "time"

// API type constants for credential profiles.
const (
	APITypeOpenAI    = "openai"
	APITypeAnthropic = "anthropic"
	APITypeGemini    = "gemini"
)

// APICredentialProfile stores a reusable Base URL + API Key pair.
type APICredentialProfile struct {
	ID             int        `json:"id" gorm:"primaryKey"`
	Name           string     `json:"name" gorm:"not null"`
	APIType        string     `json:"api_type" gorm:"not null;default:'openai'"`
	BaseURL        string     `json:"base_url" gorm:"not null"`
	APIKey         string     `json:"api_key,omitempty"`
	Tags           string     `json:"tags"`
	Notes          string     `json:"notes"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`
	HealthStatus   string     `json:"health_status" gorm:"default:'unknown'"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// APICredentialCreateRequest is the payload for creating a credential profile.
type APICredentialCreateRequest struct {
	Name    string `json:"name" binding:"required"`
	APIType string `json:"api_type"`
	BaseURL string `json:"base_url" binding:"required"`
	APIKey  string `json:"api_key" binding:"required"`
	Tags    string `json:"tags"`
	Notes   string `json:"notes"`
}

func (APICredentialCreateRequest) TableName() string { return "-" }

// APICredentialUpdateRequest is the payload for updating a credential profile.
type APICredentialUpdateRequest struct {
	ID      int     `json:"id" binding:"required"`
	Name    *string `json:"name,omitempty"`
	APIType *string `json:"api_type,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	APIKey  *string `json:"api_key,omitempty"`
	Tags    *string `json:"tags,omitempty"`
	Notes   *string `json:"notes,omitempty"`
}

func (APICredentialUpdateRequest) TableName() string { return "-" }

// VerificationRequest is the payload for running API verification probes.
type VerificationRequest struct {
	BaseURL string   `json:"base_url" binding:"required"`
	APIKey  string   `json:"api_key" binding:"required"`
	APIType string   `json:"api_type"`
	Model   string   `json:"model"`
	Probes  []string `json:"probes"` // text_gen, tool_calling, structured_output, web_search
}

func (VerificationRequest) TableName() string { return "-" }

// VerificationResult holds the outcome of running verification probes.
type VerificationResult struct {
	Probe   string `json:"probe"`
	Success bool   `json:"success"`
	Latency int64  `json:"latency_ms"`
	Error   string `json:"error,omitempty"`
	Output  string `json:"output,omitempty"`
}

// CLIExportRequest is the payload for generating CLI config snippets.
type CLIExportRequest struct {
	BaseURL string `json:"base_url" binding:"required"`
	APIKey  string `json:"api_key" binding:"required"`
	APIType string `json:"api_type"`
	Tool    string `json:"tool" binding:"required"` // claude_code, codex, gemini_cli, cherry_studio, kilo_code
}

func (CLIExportRequest) TableName() string { return "-" }

// CLIExportResult holds the generated config snippet.
type CLIExportResult struct {
	Tool        string `json:"tool"`
	Format      string `json:"format"` // json, env, toml, yaml
	Content     string `json:"content"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
}

// AllAPITypes returns the list of supported API types.
func AllAPITypes() []string {
	return []string{APITypeOpenAI, APITypeAnthropic, APITypeGemini}
}

// AllCLITools returns the list of supported CLI export tools.
func AllCLITools() []string {
	return []string{"claude_code", "codex", "gemini_cli", "cherry_studio", "kilo_code"}
}
