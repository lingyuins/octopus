// Package credential provides CRUD operations for API credential profiles.
package credential

import (
	"context"
	"fmt"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// List returns all credential profiles, with API keys masked.
func List(ctx context.Context) ([]model.APICredentialProfile, error) {
	var profiles []model.APICredentialProfile
	if err := db.GetDB().WithContext(ctx).Order("id DESC").Find(&profiles).Error; err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	for i := range profiles {
		maskKey(&profiles[i])
	}
	return profiles, nil
}

// Get returns a credential profile by ID with the key decrypted (for internal use).
func Get(ctx context.Context, id int) (*model.APICredentialProfile, error) {
	var p model.APICredentialProfile
	if err := db.GetDB().WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, fmt.Errorf("get credential %d: %w", id, err)
	}
	return &p, nil
}

// GetDecrypted returns a credential with the API key decrypted.
func GetDecrypted(ctx context.Context, id int) (*model.APICredentialProfile, error) {
	p, err := Get(ctx, id)
	if err != nil {
		return nil, err
	}
	key, err := crypto.Decrypt(p.APIKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	p.APIKey = key
	return p, nil
}

// Create persists a new credential profile.
func Create(ctx context.Context, req *model.APICredentialCreateRequest) (*model.APICredentialProfile, error) {
	encKey, err := crypto.Encrypt(req.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}

	apiType := req.APIType
	if apiType == "" {
		apiType = model.APITypeOpenAI
	}

	p := model.APICredentialProfile{
		Name:         req.Name,
		APIType:      apiType,
		BaseURL:      req.BaseURL,
		APIKey:       encKey,
		Tags:         req.Tags,
		Notes:        req.Notes,
		HealthStatus: model.HealthStatusUnknown,
	}

	if err := db.GetDB().WithContext(ctx).Create(&p).Error; err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}

	masked := p
	maskKey(&masked)
	return &masked, nil
}

// Update applies partial updates to a credential profile.
func Update(ctx context.Context, req *model.APICredentialUpdateRequest) (*model.APICredentialProfile, error) {
	if _, err := Get(ctx, req.ID); err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.APIType != nil {
		updates["api_type"] = *req.APIType
	}
	if req.BaseURL != nil {
		updates["base_url"] = *req.BaseURL
	}
	if req.APIKey != nil {
		enc, err := crypto.Encrypt(*req.APIKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt api key: %w", err)
		}
		updates["api_key"] = enc
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}

	if len(updates) > 0 {
		if err := db.GetDB().WithContext(ctx).Model(&model.APICredentialProfile{}).Where("id = ?", req.ID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update credential %d: %w", req.ID, err)
		}
	}

	updated, err := Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	maskKey(updated)
	return updated, nil
}

// Delete removes a credential profile.
func Delete(ctx context.Context, id int) error {
	result := db.GetDB().WithContext(ctx).Delete(&model.APICredentialProfile{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete credential %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("credential %d not found", id)
	}
	return nil
}

// UpdateHealth updates the health status and last verified time.
func UpdateHealth(ctx context.Context, id int, status string, verifiedAt interface{}) error {
	updates := map[string]interface{}{
		"health_status":    status,
		"last_verified_at": verifiedAt,
	}
	return db.GetDB().WithContext(ctx).Model(&model.APICredentialProfile{}).Where("id = ?", id).Updates(updates).Error
}

func maskKey(p *model.APICredentialProfile) {
	if p.APIKey != "" {
		p.APIKey = "***"
	}
}
