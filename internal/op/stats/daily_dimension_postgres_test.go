package stats

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// postgresDialectorOnly 返回一个只带方言名的 *gorm.DB，不建立连接。
// 用于断言生成的 SQL 片段，避免单测依赖真实 PostgreSQL 实例。
func postgresDialectorOnly() *gorm.DB {
	return &gorm.DB{Config: &gorm.Config{Dialector: postgres.Dialector{}}}
}

func sqliteDialectorOnly() *gorm.DB {
	return &gorm.DB{Config: &gorm.Config{Dialector: sqlite.Dialector{}}}
}

func assignmentSQL(t *testing.T, set clause.Set, column string) string {
	t.Helper()
	for _, item := range set {
		if item.Column.Name != column {
			continue
		}
		expr, ok := item.Value.(clause.Expr)
		if !ok {
			t.Fatalf("column %q value is %T, want clause.Expr", column, item.Value)
		}
		return expr.SQL
	}
	t.Fatalf("column %q not found in assignments", column)
	return ""
}

// TestDailyDimensionUpdateColumnsPostgresQualifiesTarget 回归 issue #259：
// PostgreSQL 的 ON CONFLICT DO UPDATE 里裸列名会同时匹配目标表与 excluded，
// 被判 ambiguous（SQLSTATE 42702），导致 stats_daily_* 四张每日维度表全部写入
// 失败。修复后目标表列必须带表名限定。
func TestDailyDimensionUpdateColumnsPostgresQualifiesTarget(t *testing.T) {
	dbConn := postgresDialectorOnly()

	tables := []string{
		"stats_daily_channels",
		"stats_daily_models",
		"stats_daily_api_keys",
		"stats_daily_channel_models",
	}
	columns := []string{
		"input_token",
		"output_token",
		"input_cost",
		"output_cost",
		"wait_time",
		"request_success",
		"request_failed",
		"histogram_lt100",
		"histogram100to500",
		"histogram500to1k",
		"histogram1kto5k",
		"histogram_gt5k",
	}
	maxColumns := []string{
		"latency_p50",
		"latency_p95",
		"latency_p99",
		"ftut_avg",
		"ftut_p50",
		"ftut_p95",
		"ftut_p99",
	}

	for _, table := range tables {
		set := dailyDimensionUpdateColumns(dbConn, table, false)
		for _, column := range columns {
			expr := assignmentSQL(t, set, column)
			want := table + "." + column + " + excluded." + column
			if expr != want {
				t.Errorf("%s.%s expr = %q, want %q", table, column, expr, want)
			}
		}
		for _, column := range maxColumns {
			expr := assignmentSQL(t, set, column)
			want := "GREATEST(" + table + "." + column + ", excluded." + column + ")"
			if expr != want {
				t.Errorf("%s.%s expr = %q, want %q", table, column, expr, want)
			}
		}
	}
}

// TestDailyDimensionUpdateColumnsPostgresHasNoBareTargetReference 断言修复后的
// SQL 片段里不存在裸列名累加写法（即列名前面没有表名限定），这是 42702 的直接成因。
func TestDailyDimensionUpdateColumnsPostgresHasNoBareTargetReference(t *testing.T) {
	dbConn := postgresDialectorOnly()
	set := dailyDimensionUpdateColumns(dbConn, "stats_daily_api_keys", false)

	for _, item := range set {
		expr, ok := item.Value.(clause.Expr)
		if !ok {
			continue
		}
		column := item.Column.Name
		if strings.HasPrefix(expr.SQL, column+" + ") {
			t.Errorf("column %q still uses unqualified target reference: %q", column, expr.SQL)
		}
		if strings.HasPrefix(expr.SQL, "GREATEST("+column+",") {
			t.Errorf("column %q still uses unqualified target reference: %q", column, expr.SQL)
		}
	}
}

// TestDailyDimensionUpdateColumnsSQLiteKeepsBareReference 锁住非 PostgreSQL 方言
// 的既有行为不变：SQLite/MySQL 不存在歧义问题，保持裸列名与 MAX()。
func TestDailyDimensionUpdateColumnsSQLiteKeepsBareReference(t *testing.T) {
	dbConn := sqliteDialectorOnly()
	set := dailyDimensionUpdateColumns(dbConn, "stats_daily_api_keys", false)

	if got, want := assignmentSQL(t, set, "input_token"), "input_token + excluded.input_token"; got != want {
		t.Errorf("sqlite input_token expr = %q, want %q", got, want)
	}
	if got, want := assignmentSQL(t, set, "latency_p50"), "MAX(latency_p50, excluded.latency_p50)"; got != want {
		t.Errorf("sqlite latency_p50 expr = %q, want %q", got, want)
	}
}

// TestDailyDimensionUpdateColumnsIncludesName 断言 includeName 分支同样输出
// 列名覆盖，且不被表名限定破坏（name 只从 excluded 取值，不参与累加）。
func TestDailyDimensionUpdateColumnsIncludesName(t *testing.T) {
	dbConn := postgresDialectorOnly()
	set := dailyDimensionUpdateColumns(dbConn, "stats_daily_api_keys", true)

	if got, want := assignmentSQL(t, set, "name"), "excluded.name"; got != want {
		t.Errorf("name expr = %q, want %q", got, want)
	}
}
