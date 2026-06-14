package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/lingyuins/octopus/internal/db/migrate"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB
var currentDBType string

// 独立日志库（仅承载 relay_logs）。当配置了 database.log_type/log_path 时启用，
// 否则 logDB 保持 nil，GetLogDB() 回落到主库——与旧版行为完全一致。
//
// 关闭后台日志时可调用 CloseLogDB 断开该连接（释放文件句柄/连接池），
// 重新开启时调用 ReopenLogDB 重连。这些操作只在「独立日志库」模式下有意义；
// 共用主库时它们是空操作，绝不会动主库连接。
var (
	logDB            *gorm.DB
	currentLogDBType string
	logDBType        string // 配置的日志库类型（用于 Reopen），空表示共用主库
	logDBPath        string // 配置的日志库路径（用于 Reopen）
	logDBDebug       bool
	logDBLock        sync.RWMutex
)

func IsSQLite() bool {
	return currentDBType == "sqlite"
}

// IsLogDBSeparate 报告日志是否使用独立数据库（而非共用主库）。
func IsLogDBSeparate() bool {
	logDBLock.RLock()
	defer logDBLock.RUnlock()
	return logDBType != ""
}

// IsLogSQLite 报告日志库是否为 SQLite。独立库时取日志库类型，否则取主库类型。
func IsLogSQLite() bool {
	logDBLock.RLock()
	separate := logDBType != ""
	t := currentLogDBType
	logDBLock.RUnlock()
	if separate {
		return t == "sqlite"
	}
	return IsSQLite()
}

func InitDB(dbType, dsn string, debug bool) error {
	currentDBType = dbType
	var err error
	db, err = OpenStandalone(dbType, dsn, debug)
	if err != nil {
		return err
	}
	return Migrate(db)
}

// InitLogDB 初始化独立日志库。logType/logPath 任一为空时视为「共用主库」，
// 不建立独立连接（GetLogDB 将回落到主库）。配置完整时打开独立连接并只迁移
// relay_logs 表结构。必须在 InitDB 之后调用。
func InitLogDB(logType, logPath string, debug bool) error {
	logType = strings.TrimSpace(logType)
	logPath = strings.TrimSpace(logPath)
	if logType == "postgresql" {
		logType = "postgres"
	}

	logDBLock.Lock()
	defer logDBLock.Unlock()

	// 记录配置，供 CloseLogDB/ReopenLogDB 使用。
	logDBType = logType
	logDBPath = logPath
	logDBDebug = debug

	if logType == "" || logPath == "" {
		// 共用主库：不建立独立连接。
		logDBType = ""
		logDB = nil
		currentLogDBType = ""
		return nil
	}

	conn, err := OpenStandalone(logType, logPath, debug)
	if err != nil {
		return fmt.Errorf("open log database: %w", err)
	}
	if err := MigrateLogDB(conn); err != nil {
		_ = closeConn(conn)
		return fmt.Errorf("migrate log database: %w", err)
	}
	logDB = conn
	currentLogDBType = logType
	return nil
}

// MigrateLogDB 仅迁移日志库需要的表结构（relay_logs + relay_log_attempts）。
func MigrateLogDB(conn *gorm.DB) error {
	if err := conn.AutoMigrate(&model.RelayLog{}, &model.RelayLogAttempt{}); err != nil {
		return err
	}
	if conn.Dialector != nil && conn.Dialector.Name() == "postgres" {
		conn.Exec("DEALLOCATE ALL")
		conn.Exec("DISCARD ALL")
	}
	return nil
}

// GetLogDB 返回日志库连接。独立库模式下返回独立连接；若该连接已被
// CloseLogDB 断开（logDB == nil 但仍配置了独立库），返回 nil，调用方需
// 自行判断（日志关闭场景下不应再写入）。共用主库模式下返回主库连接。
func GetLogDB() *gorm.DB {
	logDBLock.RLock()
	defer logDBLock.RUnlock()
	if logDBType != "" {
		return logDB
	}
	return db
}

// CloseLogDB 断开独立日志库连接（用于关闭后台日志时释放资源）。
// 共用主库模式下为空操作——绝不关闭主库连接。返回后 GetLogDB 在独立库
// 模式下会返回 nil，直到 ReopenLogDB 重连。
func CloseLogDB() error {
	logDBLock.Lock()
	defer logDBLock.Unlock()
	if logDBType == "" || logDB == nil {
		return nil
	}
	err := closeConn(logDB)
	logDB = nil
	currentLogDBType = ""
	return err
}

// ReopenLogDB 重新打开独立日志库连接（用于重新开启后台日志）。
// 共用主库模式下为空操作。若连接已存在则直接返回。
func ReopenLogDB() error {
	logDBLock.Lock()
	defer logDBLock.Unlock()
	if logDBType == "" {
		return nil
	}
	if logDB != nil {
		return nil
	}
	conn, err := OpenStandalone(logDBType, logDBPath, logDBDebug)
	if err != nil {
		return fmt.Errorf("reopen log database: %w", err)
	}
	if err := MigrateLogDB(conn); err != nil {
		_ = closeConn(conn)
		return fmt.Errorf("migrate log database on reopen: %w", err)
	}
	logDB = conn
	currentLogDBType = logDBType
	return nil
}

func closeConn(conn *gorm.DB) error {
	sqlDB, err := conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func OpenStandalone(dbType, dsn string, debug bool) (*gorm.DB, error) {
	gormConfig := gorm.Config{Logger: logger.Discard}
	if debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	var conn *gorm.DB
	var err error
	switch dbType {
	case "sqlite":
		conn, err = initSQLite(dsn, &gormConfig)
	case "mysql":
		conn, err = initMySQL(dsn, &gormConfig)
	case "postgres", "postgresql":
		conn, err = initPostgres(dsn, &gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	if err != nil {
		return nil, err
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	configureConnectionPool(sqlDB, dbType)
	return conn, nil
}

func Migrate(conn *gorm.DB) error {
	if err := migrate.BeforeAutoMigrate(conn); err != nil {
		return err
	}
	if err := conn.AutoMigrate(
		&model.User{},
		&model.ChannelGroup{},
		&model.Channel{},
		&model.ChannelKey{},
		&model.Group{},
		&model.GroupItem{},
		&model.AIRouteTask{},
		&model.LLMInfo{},
		&model.APIKey{},
		&model.AuditLog{},
		&model.Setting{},
		&model.StatsTotal{},
		&model.StatsDaily{},
		&model.StatsHourly{},
		&model.StatsModel{},
		&model.StatsChannel{},
		&model.StatsAPIKey{},
		&model.StatsSiteModelHourly{},
		&model.RelayLog{},
		&model.RelayLogAttempt{},
		&model.AutoStrategyState{},
		&model.CircuitBreakerState{},
		&model.AlertRule{},
		&model.AlertNotifChannel{},
		&model.AlertStateRecord{},
		&model.AlertHistory{},
		&model.RemoteSite{},
		&model.BalanceSnapshot{},
		&model.CheckInRecord{},
		&model.APICredentialProfile{},
		&model.SiteAnnouncement{},
		&model.RemoteSiteToken{},
		&model.ModelMapping{},
		&model.RedemptionRecord{},
		&model.RemoteUsageRecord{},
		&model.Site{},
		&model.SiteAccount{},
		&model.SiteToken{},
		&model.SiteUserGroup{},
		&model.SiteModel{},
		&model.SiteChannelBinding{},
		&model.ProxyConfiguration{},
		&model.WSResponseAffinity{},
		&model.WebAuthnCredential{},
		&migrate.MigrationRecord{},
	); err != nil {
		return err
	}
	if err := migrate.AfterAutoMigrate(conn); err != nil {
		return err
	}
	// Postgres: schema changes during migrations can invalidate cached prepared plans
	// (e.g. "cached plan must not change result type"). Clear them.
	if conn.Dialector != nil && conn.Dialector.Name() == "postgres" {
		conn.Exec("DEALLOCATE ALL")
		conn.Exec("DISCARD ALL")
	}
	return nil
}

func configureConnectionPool(sqlDB *sql.DB, dbType string) {
	if dbType == "sqlite" {
		// glebarez/sqlite uses a pure-Go SQLite driver. Under concurrent background
		// tasks, multiple pooled connections can surface nested transaction errors such
		// as "cannot start a transaction within a transaction". Keep SQLite on a
		// single shared connection to serialize writes safely.
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetConnMaxLifetime(0)
		sqlDB.SetConnMaxIdleTime(0)
		return
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
}

func initSQLite(path string, config *gorm.Config) (*gorm.DB, error) {
	dsn, err := sqliteDSN(path)
	if err != nil {
		return nil, err
	}
	params := []string{
		"_txlock=immediate",
		"_journal_mode=WAL",
		"_synchronous=NORMAL",
		"_cache_size=10000",
		"_busy_timeout=5000",
		"_foreign_keys=ON",
		"_auto_vacuum=INCREMENTAL",
		"_mmap_size=268435456",
		"_locking_mode=NORMAL",
	}
	db, err := gorm.Open(sqlite.Open(dsn+sqliteDSNSeparator(dsn)+strings.Join(params, "&")), config)
	if err != nil {
		return nil, wrapSQLitePathError("failed to open sqlite database", dsn, err)
	}
	return db, nil
}

func initMySQL(dsn string, config *gorm.Config) (*gorm.DB, error) {
	// DSN 格式: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	if !strings.Contains(dsn, "?") {
		dsn += "?charset=utf8mb4&parseTime=True&loc=Local"
	}
	return gorm.Open(mysql.Open(dsn), config)
}

func initPostgres(dsn string, config *gorm.Config) (*gorm.DB, error) {
	// DSN 格式: host=localhost user=postgres password=xxx dbname=octopus port=5432 sslmode=disable
	return gorm.Open(postgres.Open(dsn), config)
}

func Close() error {
	// 先关闭独立日志库（共用主库模式下为空操作），再关闭主库。
	_ = CloseLogDB()
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func GetDB() *gorm.DB {
	return db
}

// FastClearTable 以方言最快的方式清空整张表，并重建表结构与索引。
//
// 相比逐行 DELETE（百万级数据在 SQLite + WAL + 单连接下可能耗时数十分钟），
// 本函数对各方言采用近乎瞬时的清空策略：
//   - MySQL/Postgres：TRUNCATE TABLE，瞬时清空并回收空间；
//   - SQLite：DROP TABLE + AutoMigrate 重建（pure-Go 驱动下 TRUNCATE 不可用），
//     直接丢弃整张表的数据页，并通过 struct tag 完整恢复索引。
//
// 参数 model 用于 AutoMigrate 重建（SQLite），tableName 用于 TRUNCATE 拼接。
// 重建依赖 model 的 struct tag 完整声明索引；relay_logs 的 time 索引即来自
// model.RelayLog 的字段 tag，因此可被正确恢复。
func FastClearTable(conn *gorm.DB, model any, tableName string) error {
	// 方言取自连接本身，而非全局 currentDBType——日志库可能与主库不同方言。
	dialect := ""
	if conn != nil && conn.Dialector != nil {
		dialect = conn.Dialector.Name()
	}
	switch dialect {
	case "mysql":
		// MySQL TRUNCATE 是 DDL，瞬时清空并重置自增；不需要重建。
		return conn.Exec("TRUNCATE TABLE `" + tableName + "`").Error
	case "postgres", "postgresql":
		return conn.Exec(`TRUNCATE TABLE "` + tableName + `"`).Error
	default:
		// SQLite：DROP + 重建。pure-Go 驱动无 TRUNCATE，DROP 直接释放数据页，
		// 再由 AutoMigrate 依据 struct tag 重建表与索引。
		if err := conn.Migrator().DropTable(model); err != nil {
			return fmt.Errorf("drop table %s: %w", tableName, err)
		}
		if err := conn.AutoMigrate(model); err != nil {
			return fmt.Errorf("recreate table %s: %w", tableName, err)
		}
		return nil
	}
}

func sqliteDSN(path string) (string, error) {
	dsn := strings.TrimSpace(path)
	if dsn == "" {
		return "", fmt.Errorf("sqlite database path is empty")
	}
	if err := ensureSQLiteDir(dsn); err != nil {
		return "", err
	}
	return dsn, nil
}

func sqliteDSNSeparator(dsn string) string {
	if strings.Contains(dsn, "?") {
		return "&"
	}
	return "?"
}

func ensureSQLiteDir(dsn string) error {
	dbPath, ok := sqliteFilePath(dsn)
	if !ok {
		return nil
	}

	dir := filepath.Dir(dbPath)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return wrapSQLitePathError("failed to create sqlite data directory", dir, err)
	}
	return nil
}

func sqliteFilePath(dsn string) (string, bool) {
	basePath, rawQuery, _ := strings.Cut(strings.TrimSpace(dsn), "?")
	lowerBasePath := strings.ToLower(basePath)
	if basePath == ":memory:" || lowerBasePath == "file::memory:" {
		return "", false
	}
	if rawQuery != "" {
		if values, err := url.ParseQuery(rawQuery); err == nil && strings.EqualFold(values.Get("mode"), "memory") {
			return "", false
		}
	}
	if !strings.HasPrefix(lowerBasePath, "file:") {
		return basePath, true
	}

	parsed, err := url.Parse(basePath)
	if err != nil {
		return filepath.FromSlash(strings.TrimPrefix(basePath, "file:")), true
	}

	switch {
	case parsed.Path != "":
		path := parsed.Path
		if filepath.Separator == '\\' && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		if parsed.Host != "" && parsed.Host != "localhost" {
			path = "//" + parsed.Host + path
		}
		return filepath.FromSlash(path), true
	case parsed.Opaque != "":
		return filepath.FromSlash(parsed.Opaque), true
	default:
		return filepath.FromSlash(strings.TrimPrefix(basePath, "file:")), true
	}
}

func wrapSQLitePathError(action, path string, err error) error {
	if err == nil {
		return nil
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		return fmt.Errorf("%s %q: %w; make sure the sqlite path is writable by the current process (the official Docker image runs as UID/GID 1000 and needs write access to /app/data)", action, path, err)
	}
	return fmt.Errorf("%s %q: %w", action, path, err)
}
