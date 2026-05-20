package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

// settingCache is retained for backward compatibility (used by tests).
var settingCache = setting.GetCache()

// Deprecated: Use setting.List from internal/op/setting instead.
func SettingList(ctx context.Context) ([]model.Setting, error) { return setting.List(ctx) }

// Deprecated: Use setting.GetString from internal/op/setting instead.
func SettingGetString(key model.SettingKey) (string, error) { return setting.GetString(key) }

// Deprecated: Use setting.SetString from internal/op/setting instead.
func SettingSetString(key model.SettingKey, value string) error { return setting.SetString(key, value) }

// Deprecated: Use setting.GetInt from internal/op/setting instead.
func SettingGetInt(key model.SettingKey) (int, error) { return setting.GetInt(key) }

// Deprecated: Use setting.GetBool from internal/op/setting instead.
func SettingGetBool(key model.SettingKey) (bool, error) { return setting.GetBool(key) }

// Deprecated: Use setting.SetInt from internal/op/setting instead.
func SettingSetInt(key model.SettingKey, value int) error { return setting.SetInt(key, value) }

// settingRefreshCache is called by cache.go (same package)
func settingRefreshCache(ctx context.Context) error { return setting.RefreshCache(ctx) }
