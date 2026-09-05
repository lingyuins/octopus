package migrate

import (
	"fmt"

	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

func init() {
	RegisterAfterAutoMigration(Migration{
		Version: 53,
		Up:      migratePlanProviderPoolsJSON,
	})
}

// 053: 为 plan_providers 增加 pools_json 列
// （SenseNova 新版 pool-usage 接口返回分池明细，前端按官方新版额度面板
// 分池展示需要逐池的 window_5h/window_7d/grant_balance 数据）。
// 与 039 同模式：GORM AutoMigrate 通常也会加列，这里幂等兜底，确保跨方言
// （SQLite/MySQL/Postgres）以及运行时切换 DB 类型后该列存在。
func migratePlanProviderPoolsJSON(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if !db.Migrator().HasTable(&model.PlanProvider{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.PlanProvider{}, "PoolsJSON") {
		if err := db.Migrator().AddColumn(&model.PlanProvider{}, "PoolsJSON"); err != nil {
			return fmt.Errorf("add column pools_json: %w", err)
		}
	}
	return nil
}
