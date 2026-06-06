package modelmapping

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

var (
	cacheMu       sync.RWMutex
	mappingsCache []*model.ModelMapping
	compiledRegex = make(map[int]*regexp.Regexp)
)

// InitCache loads all model mappings from the database into memory.
func InitCache(ctx context.Context) error {
	var mappings []*model.ModelMapping
	if err := db.GetDB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority DESC, id ASC").
		Find(&mappings).Error; err != nil {
		return fmt.Errorf("load model mappings: %w", err)
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()

	mappingsCache = mappings
	compiledRegex = make(map[int]*regexp.Regexp)

	for _, m := range mappings {
		if m.MatchType == model.MatchRegex {
			if re, err := regexp.Compile(m.Pattern); err == nil {
				compiledRegex[m.ID] = re
			}
		}
	}

	return nil
}

// Create inserts a new model mapping and refreshes the cache.
func Create(ctx context.Context, req *model.ModelMappingCreateRequest) (*model.ModelMapping, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	mapping := &model.ModelMapping{
		Name:         req.Name,
		Pattern:      req.Pattern,
		MatchType:    req.MatchType,
		TargetModel:  req.TargetModel,
		Priority:     req.Priority,
		Enabled:      enabled,
		ScopeGroupID: req.ScopeGroupID,
	}

	if err := db.GetDB().WithContext(ctx).Create(mapping).Error; err != nil {
		return nil, fmt.Errorf("create model mapping: %w", err)
	}

	if err := InitCache(ctx); err != nil {
		return nil, err
	}

	return mapping, nil
}

// Update modifies an existing model mapping and refreshes the cache.
func Update(ctx context.Context, id int, req *model.ModelMappingUpdateRequest) (*model.ModelMapping, error) {
	var mapping model.ModelMapping
	if err := db.GetDB().WithContext(ctx).First(&mapping, id).Error; err != nil {
		return nil, fmt.Errorf("find model mapping %d: %w", id, err)
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Pattern != nil {
		updates["pattern"] = *req.Pattern
	}
	if req.MatchType != nil {
		updates["match_type"] = *req.MatchType
	}
	if req.TargetModel != nil {
		updates["target_model"] = *req.TargetModel
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.ScopeGroupID != nil {
		updates["scope_group_id"] = *req.ScopeGroupID
	}

	if len(updates) > 0 {
		if err := db.GetDB().WithContext(ctx).Model(&mapping).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update model mapping %d: %w", id, err)
		}
	}

	if err := InitCache(ctx); err != nil {
		return nil, err
	}

	return &mapping, nil
}

// Delete removes a model mapping and refreshes the cache.
func Delete(ctx context.Context, id int) error {
	if err := db.GetDB().WithContext(ctx).Delete(&model.ModelMapping{}, id).Error; err != nil {
		return fmt.Errorf("delete model mapping %d: %w", id, err)
	}

	if err := InitCache(ctx); err != nil {
		return err
	}

	return nil
}

// List returns all model mappings.
func List(ctx context.Context) ([]*model.ModelMapping, error) {
	var mappings []*model.ModelMapping
	if err := db.GetDB().WithContext(ctx).
		Order("priority DESC, id ASC").
		Find(&mappings).Error; err != nil {
		return nil, fmt.Errorf("list model mappings: %w", err)
	}
	return mappings, nil
}

// Get retrieves a single model mapping by ID.
func Get(ctx context.Context, id int) (*model.ModelMapping, error) {
	var mapping model.ModelMapping
	if err := db.GetDB().WithContext(ctx).First(&mapping, id).Error; err != nil {
		return nil, fmt.Errorf("get model mapping %d: %w", id, err)
	}
	return &mapping, nil
}

// Resolve applies model mapping rules to transform a request model name.
// It checks rules in priority order and returns the first match.
// If no match is found, returns the original requestModel unchanged.
func Resolve(ctx context.Context, requestModel string, groupID int) string {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	for _, m := range mappingsCache {
		if m.ScopeGroupID != nil && *m.ScopeGroupID != groupID {
			continue
		}

		if matchPattern(m, requestModel) {
			return m.TargetModel
		}
	}

	return requestModel
}

// matchPattern checks if the requestModel matches the mapping pattern.
// Must be called while holding cacheMu.RLock().
func matchPattern(m *model.ModelMapping, requestModel string) bool {
	switch m.MatchType {
	case model.MatchExact:
		return strings.EqualFold(requestModel, m.Pattern)

	case model.MatchWildcard:
		return matchWildcard(m.Pattern, requestModel)

	case model.MatchRegex:
		re, ok := compiledRegex[m.ID]
		if !ok {
			return false // regex failed to compile during InitCache
		}
		return re.MatchString(requestModel)

	default:
		return false
	}
}

// matchWildcard implements glob-style pattern matching.
// '*' matches any sequence of characters, '?' matches any single character.
func matchWildcard(pattern, s string) bool {
	pattern = strings.ToLower(pattern)
	s = strings.ToLower(s)

	pIdx, sIdx := 0, 0
	starIdx, match := -1, 0

	for sIdx < len(s) {
		if pIdx < len(pattern) && (pattern[pIdx] == '?' || pattern[pIdx] == s[sIdx]) {
			pIdx++
			sIdx++
		} else if pIdx < len(pattern) && pattern[pIdx] == '*' {
			starIdx = pIdx
			match = sIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			match++
			sIdx = match
		} else {
			return false
		}
	}

	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == len(pattern)
}
