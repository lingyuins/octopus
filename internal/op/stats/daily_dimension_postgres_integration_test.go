package stats

import (
	"context"
	"os"
	"testing"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
)

// TestDailyDimensionPostgresIntegration 对着真实 PostgreSQL 跑一遍四张每日
// 维度表的 upsert，验证修复后确实能落库并累加。默认跳过，需要时用
// OCTOPUS_TEST_POSTGRES_DSN 指向一个可写的测试库：
//
//	OCTOPUS_TEST_POSTGRES_DSN=postgresql://user:pass@127.0.0.1:5432/db go test ./internal/op/stats/ -run PostgresIntegration -v
func TestDailyDimensionPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("OCTOPUS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("OCTOPUS_TEST_POSTGRES_DSN not set")
	}
	if err := db.InitDB("postgres", dsn, false); err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	// 幂等：先清掉上一轮可能残留的测试行，否则累加值会跨轮叠加。
	if err := db.GetDB().WithContext(ctx).Where("channel_id = ? OR channel_id = ?", 9001, 0).Delete(&model.StatsDailyChannel{}).Error; err != nil {
		t.Fatalf("clean daily channel: %v", err)
	}
	if err := db.GetDB().WithContext(ctx).Where("model_name = ?", "m9001").Delete(&model.StatsDailyModel{}).Error; err != nil {
		t.Fatalf("clean daily model: %v", err)
	}
	if err := db.GetDB().WithContext(ctx).Where("api_key_id = ?", 9001).Delete(&model.StatsDailyAPIKey{}).Error; err != nil {
		t.Fatalf("clean daily api key: %v", err)
	}
	if err := db.GetDB().WithContext(ctx).Where("channel_id = ?", 9001).Delete(&model.StatsDailyChannelModel{}).Error; err != nil {
		t.Fatalf("clean daily channel model: %v", err)
	}
	m := model.StatsMetrics{RequestSuccess: 1, InputCost: 0.14, OutputCost: 0.28, InputToken: 10, OutputToken: 20}
	for i := 0; i < 2; i++ {
		if err := DailyDimensionAPIKeyUpdate(ctx, 9001, "k9001", m); err != nil {
			t.Fatalf("api key upsert #%d: %v", i+1, err)
		}
		if err := DailyDimensionChannelUpdate(ctx, 9001, "ch9001", m); err != nil {
			t.Fatalf("channel upsert #%d: %v", i+1, err)
		}
		if err := DailyDimensionModelUpdate(ctx, "m9001", m); err != nil {
			t.Fatalf("model upsert #%d: %v", i+1, err)
		}
		if err := DailyDimensionChannelModelUpdate(ctx, 9001, "ch9001", "m9001", m); err != nil {
			t.Fatalf("channel-model upsert #%d: %v", i+1, err)
		}
	}
	var row model.StatsDailyAPIKey
	if err := db.GetDB().WithContext(ctx).Where("api_key_id = ? AND date = ?", 9001, today()).First(&row).Error; err != nil {
		t.Fatalf("stats_daily_api_keys row missing: %v", err)
	}
	if row.RequestSuccess != 2 || row.InputToken != 20 {
		t.Fatalf("row = %+v, want success=2 input_token=20", row.StatsMetrics)
	}
}
