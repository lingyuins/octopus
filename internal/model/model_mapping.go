package model

import "time"

// MatchType defines how the Pattern field is matched against request model names.
type MatchType string

const (
	MatchExact    MatchType = "exact"    // Exact string match (case-insensitive)
	MatchWildcard MatchType = "wildcard" // Glob-style pattern matching (* and ?)
	MatchRegex    MatchType = "regex"    // Regular expression matching
)

// ModelMapping defines a global rule to rewrite request model names
// before they are resolved to upstream channels.
type ModelMapping struct {
	ID           int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"size:255;not null"`         // Human-readable name
	Pattern      string    `json:"pattern" gorm:"size:512;not null"`      // Match pattern
	MatchType    MatchType `json:"match_type" gorm:"size:20;not null"`    // exact/wildcard/regex
	TargetModel  string    `json:"target_model" gorm:"size:255;not null"` // Model name to rewrite to
	Priority     int       `json:"priority" gorm:"default:0"`             // Higher priority matches first
	Enabled      bool      `json:"enabled" gorm:"default:true"`           // Whether this mapping is active
	ScopeGroupID *int      `json:"scope_group_id" gorm:"index"`           // Optional: only apply to specific group
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ModelMappingCreateRequest is the payload for creating a new model mapping.
type ModelMappingCreateRequest struct {
	Name         string    `json:"name" binding:"required"`
	Pattern      string    `json:"pattern" binding:"required"`
	MatchType    MatchType `json:"match_type" binding:"required"`
	TargetModel  string    `json:"target_model" binding:"required"`
	Priority     int       `json:"priority"`
	Enabled      *bool     `json:"enabled"`
	ScopeGroupID *int      `json:"scope_group_id"`
}

func (ModelMappingCreateRequest) TableName() string { return "-" }

// ModelMappingUpdateRequest is the payload for updating an existing model mapping.
type ModelMappingUpdateRequest struct {
	Name         *string    `json:"name"`
	Pattern      *string    `json:"pattern"`
	MatchType    *MatchType `json:"match_type"`
	TargetModel  *string    `json:"target_model"`
	Priority     *int       `json:"priority"`
	Enabled      *bool      `json:"enabled"`
	ScopeGroupID *int       `json:"scope_group_id"`
}

func (ModelMappingUpdateRequest) TableName() string { return "-" }
