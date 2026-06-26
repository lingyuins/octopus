package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestConfigureConnectionPoolLimitsSQLiteConnections(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer sqlDB.Close()

	configureConnectionPool(sqlDB, "sqlite")

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPoolLimitsPostgresConnections(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer sqlDB.Close()

	configureConnectionPool(sqlDB, "postgres")

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Fatalf("MaxOpenConnections = %d, want 5", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPoolUsesPostgresEnvOverride(t *testing.T) {
	t.Setenv("OCTOPUS_DB_MAX_OPEN_CONNS", "3")

	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer sqlDB.Close()

	configureConnectionPool(sqlDB, "postgresql")

	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", stats.MaxOpenConnections)
	}
}

func TestInitSQLiteCreatesParentDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "octopus.db")

	gdb, err := initSQLite(dbPath, &gorm.Config{Logger: logger.Discard}, DefaultSQLiteOptions())
	if err != nil {
		t.Fatalf("initSQLite() error = %v", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("gdb.DB() error = %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("sqlDB.Ping() error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("os.Stat(parent dir) error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("os.Stat(db file) error = %v", err)
	}
}

func TestSQLiteDSNAppendsParamsWithExistingQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "octopus.db") + "?_txlock=immediate"

	dsn, err := sqliteDSN(dbPath)
	if err != nil {
		t.Fatalf("sqliteDSN() error = %v", err)
	}
	got := dsn + sqliteDSNSeparator(dsn) + "_journal_mode=WAL"
	if !strings.Contains(got, "?_txlock=immediate&_journal_mode=WAL") {
		t.Fatalf("combined DSN = %q, want query parameters appended with '&'", got)
	}
}

func TestInitSQLiteCreatesParentDirForFileURI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "octopus.db")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_txlock=immediate"

	gdb, err := initSQLite(dsn, &gorm.Config{Logger: logger.Discard}, DefaultSQLiteOptions())
	if err != nil {
		t.Fatalf("initSQLite() error = %v", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("gdb.DB() error = %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("sqlDB.Ping() error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("os.Stat(parent dir) error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("os.Stat(db file) error = %v", err)
	}
}

func TestSQLiteDSNSkipsDirCreationForMemoryFileURI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "octopus.db")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=memory&cache=shared"

	got, err := sqliteDSN(dsn)
	if err != nil {
		t.Fatalf("sqliteDSN() error = %v", err)
	}
	if got != dsn {
		t.Fatalf("sqliteDSN() = %q, want %q", got, dsn)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); !os.IsNotExist(err) {
		t.Fatalf("expected memory sqlite DSN not to create parent dir, stat error = %v", err)
	}
}

func resetLogDBStateForTest(t *testing.T) {
	t.Helper()
	logDBLock.Lock()
	if logDB != nil {
		_ = closeConn(logDB)
	}
	logDB = nil
	logDBType = ""
	logDBPath = ""
	currentLogDBType = ""
	logDBDebug = false
	logDBLock.Unlock()
}

func TestInitLogDBSharedFallsBackToMainDB(t *testing.T) {
	resetLogDBStateForTest(t)
	mainPath := filepath.Join(t.TempDir(), "main.db")
	if err := InitDB("sqlite", mainPath, false); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() { _ = Close(); resetLogDBStateForTest(t) })

	// Empty log config => shared with main DB.
	if err := InitLogDB("", "", false); err != nil {
		t.Fatalf("InitLogDB() error = %v", err)
	}
	if IsLogDBSeparate() {
		t.Fatalf("IsLogDBSeparate() = true, want false for empty config")
	}
	if GetLogDB() != GetDB() {
		t.Fatalf("GetLogDB() should return the main DB in shared mode")
	}
	// Close/reopen are no-ops in shared mode and must never nil out the main DB.
	if err := CloseLogDB(); err != nil {
		t.Fatalf("CloseLogDB() shared no-op error = %v", err)
	}
	if GetLogDB() != GetDB() {
		t.Fatalf("GetLogDB() should still return main DB after no-op CloseLogDB")
	}
}

func TestInitLogDBSeparateRoutesAndLifecycle(t *testing.T) {
	resetLogDBStateForTest(t)
	mainPath := filepath.Join(t.TempDir(), "main.db")
	if err := InitDB("sqlite", mainPath, false); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "logs.db")
	if err := InitLogDB("sqlite", logPath, false); err != nil {
		t.Fatalf("InitLogDB() error = %v", err)
	}
	t.Cleanup(func() { _ = Close(); resetLogDBStateForTest(t) })

	if !IsLogDBSeparate() {
		t.Fatalf("IsLogDBSeparate() = false, want true for separate config")
	}
	logConn := GetLogDB()
	if logConn == nil {
		t.Fatalf("GetLogDB() = nil, want separate connection")
	}
	if logConn == GetDB() {
		t.Fatalf("GetLogDB() must differ from main DB in separate mode")
	}

	// relay_logs lives on the log DB; the log file must exist after migration.
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log DB file not created: %v", err)
	}

	// Close drops the connection; GetLogDB returns nil (callers must guard).
	if err := CloseLogDB(); err != nil {
		t.Fatalf("CloseLogDB() error = %v", err)
	}
	if GetLogDB() != nil {
		t.Fatalf("GetLogDB() should be nil after CloseLogDB in separate mode")
	}
	if !IsLogDBSeparate() {
		t.Fatalf("IsLogDBSeparate() should remain true after close (config retained)")
	}

	// Reopen restores a working connection.
	if err := ReopenLogDB(); err != nil {
		t.Fatalf("ReopenLogDB() error = %v", err)
	}
	if GetLogDB() == nil {
		t.Fatalf("GetLogDB() = nil after ReopenLogDB, want connection")
	}
	// Main DB must remain usable throughout.
	if GetDB() == nil {
		t.Fatalf("main DB nil after log DB lifecycle")
	}
}

// TestSQLitePragmaParamsShape 断言 DSN 参数使用驱动真正生效的 _pragma=xxx(yyy)
// 格式，而非此前被 glebarez/go-sqlite 静默忽略的 _cache_size=/_mmap_size= 形式
// （见 issue #97：低内存环境下配置从未生效）。
func TestSQLitePragmaParamsShape(t *testing.T) {
	params := sqlitePragmaParams(SQLiteOptions{CacheSize: -20000, MMapSize: 0})
	joined := strings.Join(params, "&")

	required := []string{
		"_txlock=immediate",
		"_pragma=journal_mode(WAL)",
		"_pragma=synchronous(NORMAL)",
		"_pragma=cache_size(-20000)",
		"_pragma=mmap_size(0)",
		"_pragma=busy_timeout(5000)",
		"_pragma=foreign_keys(ON)",
		"_pragma=auto_vacuum(INCREMENTAL)",
	}
	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("sqlitePragmaParams() = %q, missing %q", joined, want)
		}
	}

	// 旧格式（被驱动静默忽略）必须绝迹，否则 cache_size/mmap_size 配置形同虚设。
	for _, bad := range []string{"_cache_size=", "_mmap_size=", "_journal_mode=", "_synchronous=", "_foreign_keys=", "_locking_mode="} {
		if strings.Contains(joined, bad) {
			t.Fatalf("sqlitePragmaParams() = %q, must not contain ignored key %q", joined, bad)
		}
	}
}

// TestSQLitePragmaParamsReflectsOptions 断言 cache_size / mmap_size 随 opts 变化，
// 且 CacheSize=0 时回落到 DefaultSQLiteCacheSize（≈20MB，低内存安全默认）。
func TestSQLitePragmaParamsReflectsOptions(t *testing.T) {
	cases := []struct {
		name      string
		opts      SQLiteOptions
		wantCache string
		wantMMap  string
	}{
		{"zero falls back to default", SQLiteOptions{}, "_pragma=cache_size(" + strconv.Itoa(DefaultSQLiteCacheSize) + ")", "_pragma=mmap_size(0)"},
		{"explicit cache and mmap", SQLiteOptions{CacheSize: -50000, MMapSize: 268435456}, "_pragma=cache_size(-50000)", "_pragma=mmap_size(268435456)"},
		{"mmap disabled", SQLiteOptions{CacheSize: -10000, MMapSize: 0}, "_pragma=cache_size(-10000)", "_pragma=mmap_size(0)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			joined := strings.Join(sqlitePragmaParams(tc.opts), "&")
			if !strings.Contains(joined, tc.wantCache) {
				t.Fatalf("got %q, want cache %q", joined, tc.wantCache)
			}
			if !strings.Contains(joined, tc.wantMMap) {
				t.Fatalf("got %q, want mmap %q", joined, tc.wantMMap)
			}
		})
	}
}

// TestInitSQLiteAppliesConfiguredPragmas 是修复的核心端到端证明：打开真实 SQLite
// 后查询 PRAGMA，断言返回值等于配置值。在旧代码（_cache_size= 形式被忽略）下，
// cache_size 会停在 SQLite 默认的 2000、mmap_size 停在 0、synchronous 停在 FULL——
// 这些断言会失败；修复后必须通过。
func TestInitSQLiteAppliesConfiguredPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pragma.db")
	gdb, err := initSQLite(dbPath, &gorm.Config{Logger: logger.Discard}, SQLiteOptions{CacheSize: -40000, MMapSize: 0})
	if err != nil {
		t.Fatalf("initSQLite() error = %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("gdb.DB() error = %v", err)
	}
	defer sqlDB.Close()

	// cache_size：SQLite 对负值返回 KB 数（-40000 → -40000）。
	// mmap_size：0 表示禁用。
	// synchronous：NORMAL=1。journal_mode：WAL。
	// foreign_keys：ON=1。busy_timeout：5000ms。
	type pragmaCase struct {
		query string
		want  int64
	}
	for _, tc := range []pragmaCase{
		{"PRAGMA cache_size", -40000},
		{"PRAGMA mmap_size", 0},
		{"PRAGMA synchronous", 1},
		{"PRAGMA foreign_keys", 1},
		{"PRAGMA busy_timeout", 5000},
	} {
		var got int64
		if err := sqlDB.QueryRow(tc.query).Scan(&got); err != nil {
			t.Fatalf("%s scan error = %v", tc.query, err)
		}
		if got != tc.want {
			t.Fatalf("%s = %d, want %d (PRAGMA not actually applied — see issue #97)", tc.query, got, tc.want)
		}
	}

	// journal_mode 返回字符串 "wal"。
	var journal string
	if err := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("journal_mode scan error = %v", err)
	}
	if !strings.EqualFold(journal, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journal)
	}
}

// TestInitDBWithOptionsAppliesPragmaToMainDB 验证完整入口 InitDBWithOptions 也把
// cache_size 下发到主库连接（启动路径用的就是这个函数）。
func TestInitDBWithOptionsAppliesPragmaToMainDB(t *testing.T) {
	mainPath := filepath.Join(t.TempDir(), "main.db")
	if err := InitDBWithOptions("sqlite", mainPath, false, SQLiteOptions{CacheSize: -30000, MMapSize: 0}); err != nil {
		t.Fatalf("InitDBWithOptions() error = %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	sqlDB, err := GetDB().DB()
	if err != nil {
		t.Fatalf("GetDB().DB() error = %v", err)
	}
	var got int64
	if err := sqlDB.QueryRow("PRAGMA cache_size").Scan(&got); err != nil {
		t.Fatalf("cache_size scan error = %v", err)
	}
	if got != -30000 {
		t.Fatalf("PRAGMA cache_size = %d, want -30000", got)
	}
}
