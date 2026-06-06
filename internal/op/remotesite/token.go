package remotesite

import (
	"context"
	"fmt"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/hub"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/channel"
	"github.com/lingyuins/octopus/internal/transformer/outbound"
	"github.com/lingyuins/octopus/internal/utils/crypto"
	"github.com/lingyuins/octopus/internal/utils/log"
)

// SyncTokens fetches tokens from the remote site and upserts the local cache.
func SyncTokens(ctx context.Context, siteID int) (int, error) {
	site, err := Get(ctx, siteID)
	if err != nil {
		return 0, err
	}

	adapter, err := hub.Get(site.SiteType)
	if err != nil {
		return 0, err
	}

	remoteTokens, err := adapter.FetchTokens(ctx, site)
	if err != nil {
		return 0, fmt.Errorf("fetch tokens: %w", err)
	}

	tx := db.GetDB().WithContext(ctx).Begin()
	tx.Where("remote_site_id = ?", siteID).Delete(&model.RemoteSiteToken{})

	now := time.Now()
	count := 0
	for _, rt := range remoteTokens {
		encryptedKey, encErr := crypto.Encrypt(rt.Key)
		if encErr != nil {
			log.Warnf("encrypt token key: %v", encErr)
			continue
		}
		token := model.RemoteSiteToken{
			RemoteSiteID:   siteID,
			RemoteTokenID:  rt.ID,
			Name:           rt.Name,
			Key:            encryptedKey,
			Status:         rt.Status,
			RemainQuota:    rt.RemainQuota,
			UsedQuota:      rt.UsedQuota,
			UnlimitedQuota: rt.UnlimitedQuota,
			ModelLimits:    rt.ModelLimits,
			ExpiredTime:    rt.ExpiredTime,
			CreatedTime:    rt.CreatedTime,
			LastSyncAt:     &now,
		}
		if err := tx.Create(&token).Error; err != nil {
			log.Warnf("store remote token: %v", err)
			continue
		}
		count++
	}
	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("commit tokens: %w", err)
	}
	return count, nil
}

// ListTokens returns cached remote tokens for a site.
func ListTokens(ctx context.Context, siteID int) ([]model.RemoteSiteToken, error) {
	var tokens []model.RemoteSiteToken
	if err := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("id ASC").
		Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	for i := range tokens {
		if tokens[i].Key != "" {
			tokens[i].Key = "***"
		}
	}
	return tokens, nil
}

// GetTokenDecrypted returns a single remote token with decrypted key.
func GetTokenDecrypted(ctx context.Context, tokenID int64) (*model.RemoteSiteToken, error) {
	var token model.RemoteSiteToken
	if err := db.GetDB().WithContext(ctx).First(&token, tokenID).Error; err != nil {
		return nil, fmt.Errorf("get token %d: %w", tokenID, err)
	}
	decrypted, err := crypto.Decrypt(token.Key)
	if err != nil {
		return nil, fmt.Errorf("decrypt token key: %w", err)
	}
	token.Key = decrypted
	return &token, nil
}

// SyncToChannel imports a remote token as a local Octopus channel.
func SyncToChannel(ctx context.Context, req *model.SyncToChannelRequest) (*model.Channel, error) {
	token, err := GetTokenDecrypted(ctx, req.TokenID)
	if err != nil {
		return nil, err
	}

	site, err := Get(ctx, req.RemoteSiteID)
	if err != nil {
		return nil, err
	}

	name := req.ChannelName
	if name == "" {
		name = fmt.Sprintf("%s-%s", site.Name, token.Name)
	}

	ch := &model.Channel{
		Name:    name,
		Type:    outbound.OutboundTypeOpenAIChat,
		Enabled: true,
		BaseUrls: []model.BaseUrl{
			{URL: site.BaseURL},
		},
		Model: req.Models,
		Keys: []model.ChannelKey{
			{
				Enabled:    true,
				ChannelKey: token.Key,
				Remark:     fmt.Sprintf("Imported from %s (token: %s)", site.Name, token.Name),
			},
		},
	}

	if err := channel.Create(ch, ctx); err != nil {
		return nil, fmt.Errorf("create channel from token: %w", err)
	}
	return ch, nil
}

// BatchExportToken is a single token entry in the batch export output.
type BatchExportToken struct {
	Name           string  `json:"name"`
	Key            string  `json:"key"`
	BaseURL        string  `json:"base_url"`
	Status         int     `json:"status"`
	RemainQuota    float64 `json:"remain_quota"`
	UsedQuota      float64 `json:"used_quota"`
	UnlimitedQuota bool    `json:"unlimited_quota"`
	ModelLimits    string  `json:"model_limits,omitempty"`
	ExpiredTime    int64   `json:"expired_time"`
}

// BatchExportResult is the full payload returned by BatchExportTokens.
type BatchExportResult struct {
	SiteName     string             `json:"site_name"`
	SiteBaseURL  string             `json:"site_base_url"`
	ExportedAt   time.Time          `json:"exported_at"`
	TotalTokens  int                `json:"total_tokens"`
	ActiveTokens int                `json:"active_tokens"`
	Tokens       []BatchExportToken `json:"tokens"`
}

// BatchExportTokens returns all cached tokens for a site with decrypted keys,
// suitable for export as a JSON file.
func BatchExportTokens(ctx context.Context, siteID int) (*BatchExportResult, error) {
	site, err := Get(ctx, siteID)
	if err != nil {
		return nil, err
	}

	var tokens []model.RemoteSiteToken
	if err := db.GetDB().WithContext(ctx).
		Where("remote_site_id = ?", siteID).
		Order("id ASC").
		Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}

	result := &BatchExportResult{
		SiteName:    site.Name,
		SiteBaseURL: site.BaseURL,
		ExportedAt:  time.Now(),
		Tokens:      make([]BatchExportToken, 0, len(tokens)),
	}

	for _, t := range tokens {
		decrypted, decErr := crypto.Decrypt(t.Key)
		if decErr != nil {
			log.Warnf("decrypt token %d for export: %v", t.ID, decErr)
			continue
		}
		entry := BatchExportToken{
			Name:           t.Name,
			Key:            decrypted,
			BaseURL:        site.BaseURL,
			Status:         t.Status,
			RemainQuota:    t.RemainQuota,
			UsedQuota:      t.UsedQuota,
			UnlimitedQuota: t.UnlimitedQuota,
			ModelLimits:    t.ModelLimits,
			ExpiredTime:    t.ExpiredTime,
		}
		if entry.Status == 1 {
			result.ActiveTokens++
		}
		result.Tokens = append(result.Tokens, entry)
	}
	result.TotalTokens = len(result.Tokens)
	return result, nil
}

// SyncAllTokens fetches tokens for all enabled sites.
func SyncAllTokens(ctx context.Context) int {
	var sites []model.RemoteSite
	if err := db.GetDB().WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		log.Warnf("list sites for token sync: %v", err)
		return 0
	}
	count := 0
	for _, site := range sites {
		n, err := SyncTokens(ctx, site.ID)
		if err != nil {
			log.Warnf("sync tokens for site %d: %v", site.ID, err)
			continue
		}
		count += n
	}
	return count
}
