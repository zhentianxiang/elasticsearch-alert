package database

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"opensearch-alert/pkg/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// Database 数据库连接
type Database struct {
	db     *sql.DB
	logger *logrus.Logger
	dbType string
}

// NewDatabase 创建数据库连接
func NewDatabase(config types.DatabaseConfig, logger *logrus.Logger) (*Database, error) {
	// 确保数据库目录存在
	var dsn string
	driver := "sqlite3"
	if config.Type == "mysql" {
		// MySQL 8.0+ DSN
		if config.Params == "" {
			config.Params = "charset=utf8mb4&parseTime=true&loc=Local"
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", config.Username, config.Password, config.Host, config.Port, config.DBName, config.Params)
		driver = "mysql"
	} else {
		dbDir := filepath.Dir(config.Path)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据库目录失败: %w", err)
		}
		dsn = config.Path
	}

	// 连接数据库
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	database := &Database{
		db:     db,
		logger: logger,
		dbType: config.Type,
	}

	// 初始化表结构
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	logger.Info("数据库连接成功")
	return database, nil
}

// initTables 初始化数据库表
func (d *Database) initTables() error {
	if d.dbType == "mysql" {
		// MySQL 8.0+
		createAlertHistoryTable := `
        CREATE TABLE IF NOT EXISTS alert_history (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            alert_id VARCHAR(191) NOT NULL,
            rule_name VARCHAR(255) NOT NULL,
            level VARCHAR(32) NOT NULL,
            message TEXT NOT NULL,
            timestamp DATETIME NOT NULL,
            data TEXT,
            count BIGINT NOT NULL,
            matches BIGINT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`
		if _, err := d.db.Exec(createAlertHistoryTable); err != nil {
			return fmt.Errorf("创建告警历史表失败: %w", err)
		}

		createSessionTable := `
        CREATE TABLE IF NOT EXISTS user_sessions (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            session_id VARCHAR(191) UNIQUE NOT NULL,
            username VARCHAR(255) NOT NULL,
            role VARCHAR(32) NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            expires_at DATETIME NOT NULL
        )`
		if _, err := d.db.Exec(createSessionTable); err != nil {
			return fmt.Errorf("创建用户会话表失败: %w", err)
		}

		// 规则锁表（租约锁）
		createRuleLockTable := `
        CREATE TABLE IF NOT EXISTS rule_locks (
            rule_name VARCHAR(255) PRIMARY KEY,
            locked_by VARCHAR(255) NOT NULL DEFAULT '',
            locked_at TIMESTAMP NULL,
            ttl_seconds INT NOT NULL DEFAULT 30
        )`
		if _, err := d.db.Exec(createRuleLockTable); err != nil {
			return fmt.Errorf("创建规则锁表失败: %w", err)
		}

		// 去重表：记录最近一次发送的告警签名
		createDedupeTable := `
        CREATE TABLE IF NOT EXISTS alert_dedupe (
            dedupe_key VARCHAR(255) PRIMARY KEY,
            alert_id VARCHAR(191) NOT NULL,
            rule_name VARCHAR(255) NOT NULL,
            level VARCHAR(32) NOT NULL,
            message_hash VARCHAR(64) NOT NULL,
            last_sent DATETIME NOT NULL,
            ttl_seconds INT NOT NULL DEFAULT 120
        )`
		if _, err := d.db.Exec(createDedupeTable); err != nil {
			return fmt.Errorf("创建去重表失败: %w", err)
		}

		// MySQL 不支持 CREATE INDEX IF NOT EXISTS，这里直接创建并忽略已存在错误(1061)
		indexes := []string{
			"CREATE INDEX idx_alert_id ON alert_history(alert_id)",
			"CREATE INDEX idx_rule_name ON alert_history(rule_name)",
			"CREATE INDEX idx_level ON alert_history(level)",
			"CREATE INDEX idx_timestamp ON alert_history(timestamp)",
			"CREATE INDEX idx_session_id ON user_sessions(session_id)",
			"CREATE INDEX idx_username ON user_sessions(username)",
		}
		for _, indexSQL := range indexes {
			if _, err := d.db.Exec(indexSQL); err != nil {
				// Duplicate key name -> 1061, 或者错误信息包含 "exists"
				if strings.Contains(err.Error(), "1061") || strings.Contains(strings.ToLower(err.Error()), "exists") {
					continue
				}
				d.logger.Warnf("创建索引失败: %v", err)
			}
		}
	} else {
		// SQLite
		createAlertHistoryTable := `
        CREATE TABLE IF NOT EXISTS alert_history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            alert_id TEXT NOT NULL,
            rule_name TEXT NOT NULL,
            level TEXT NOT NULL,
            message TEXT NOT NULL,
            timestamp DATETIME NOT NULL,
            data TEXT,
            count INTEGER NOT NULL,
            matches INTEGER NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`
		if _, err := d.db.Exec(createAlertHistoryTable); err != nil {
			return fmt.Errorf("创建告警历史表失败: %w", err)
		}

		createSessionTable := `
        CREATE TABLE IF NOT EXISTS user_sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            session_id TEXT UNIQUE NOT NULL,
            username TEXT NOT NULL,
            role TEXT NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            expires_at DATETIME NOT NULL
        )`
		if _, err := d.db.Exec(createSessionTable); err != nil {
			return fmt.Errorf("创建用户会话表失败: %w", err)
		}

		// 规则锁表（租约锁）
		createRuleLockTable := `
        CREATE TABLE IF NOT EXISTS rule_locks (
            rule_name TEXT PRIMARY KEY,
            locked_by TEXT NOT NULL DEFAULT '',
            locked_at DATETIME,
            ttl_seconds INTEGER NOT NULL DEFAULT 30
        )`
		if _, err := d.db.Exec(createRuleLockTable); err != nil {
			return fmt.Errorf("创建规则锁表失败: %w", err)
		}

		// 去重表
		createDedupeTable := `
        CREATE TABLE IF NOT EXISTS alert_dedupe (
            dedupe_key TEXT PRIMARY KEY,
            alert_id TEXT NOT NULL,
            rule_name TEXT NOT NULL,
            level TEXT NOT NULL,
            message_hash TEXT NOT NULL,
            last_sent DATETIME NOT NULL,
            ttl_seconds INTEGER NOT NULL DEFAULT 120
        )`
		if _, err := d.db.Exec(createDedupeTable); err != nil {
			return fmt.Errorf("创建去重表失败: %w", err)
		}

		indexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_alert_id ON alert_history(alert_id)",
			"CREATE INDEX IF NOT EXISTS idx_rule_name ON alert_history(rule_name)",
			"CREATE INDEX IF NOT EXISTS idx_level ON alert_history(level)",
			"CREATE INDEX IF NOT EXISTS idx_timestamp ON alert_history(timestamp)",
			"CREATE INDEX IF NOT EXISTS idx_session_id ON user_sessions(session_id)",
			"CREATE INDEX IF NOT EXISTS idx_username ON user_sessions(username)",
		}
		for _, indexSQL := range indexes {
			if _, err := d.db.Exec(indexSQL); err != nil {
				d.logger.Warnf("创建索引失败: %v", err)
			}
		}
	}
	d.logger.Info("数据库表初始化完成")
	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}

// SaveAlert 保存告警记录
func (d *Database) SaveAlert(alert *types.Alert) error {
	dataJSON, err := json.Marshal(alert.Data)
	if err != nil {
		return fmt.Errorf("序列化告警数据失败: %w", err)
	}

	query := `
	INSERT INTO alert_history (alert_id, rule_name, level, message, timestamp, data, count, matches)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = d.db.Exec(query,
		alert.ID,
		alert.RuleName,
		alert.Level,
		alert.Message,
		alert.Timestamp,
		string(dataJSON),
		alert.Count,
		alert.Matches,
	)

	if err != nil {
		return fmt.Errorf("保存告警记录失败: %w", err)
	}

	d.logger.Debugf("告警记录已保存: %s", alert.ID)
	return nil
}

// GetAlertStats 获取告警统计
func (d *Database) GetAlertStats(hours int) (*types.AlertStats, error) {
	// 初始化统计结构
	stats := &types.AlertStats{
		LevelStats:   make(map[string]int64),
		RecentAlerts: []types.AlertHistory{},
	}

	// 计算时间范围
	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	// 1. 获取总告警数
	err := d.db.QueryRow("SELECT COUNT(*) FROM alert_history WHERE timestamp >= ?", startTime).Scan(&stats.TotalAlerts)
	if err != nil && err != sql.ErrNoRows {
		d.logger.Errorf("获取总告警数失败: %v", err)
		return nil, err
	}

	// 2. 获取各级别告警数
	levelQuery := "SELECT level, COUNT(*) as count FROM alert_history WHERE timestamp >= ? GROUP BY level"
	rows, err := d.db.Query(levelQuery, startTime)
	if err != nil {
		d.logger.Errorf("获取各级别告警数失败: %v", err)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var level string
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			d.logger.Errorf("扫描告警级别统计失败: %v", err)
			continue
		}
		stats.LevelStats[level] = count
	}

	// 3. 获取每小时告警统计（使用本地时区）
	var hourlyStatsQuery string
	if d.dbType == "mysql" {
		hourlyStatsQuery = `
            SELECT DATE_FORMAT(timestamp, '%H') as hour, COUNT(*) as count
            FROM alert_history
            WHERE timestamp >= ?
            GROUP BY hour
            ORDER BY hour`
	} else {
		hourlyStatsQuery = `
            SELECT strftime('%H', timestamp, 'localtime') as hour, COUNT(*) as count
            FROM alert_history
            WHERE timestamp >= ?
            GROUP BY hour
            ORDER BY hour`
	}
	rows, err = d.db.Query(hourlyStatsQuery, startTime)
	if err != nil {
		d.logger.Errorf("获取每小时告警统计失败: %v", err)
		return nil, err
	}
	defer rows.Close()

	var hourlyStats []types.HourlyStat
	for rows.Next() {
		var hs types.HourlyStat
		var hourStr string
		if err := rows.Scan(&hourStr, &hs.Count); err != nil {
			d.logger.Errorf("扫描每小时告警统计失败: %v", err)
			continue
		}
		hs.Hour, _ = strconv.Atoi(hourStr)
		hourlyStats = append(hourlyStats, hs)
	}
	stats.HourlyStats = hourlyStats

	// 4. 获取最近的告警
	recentAlertsQuery := "SELECT * FROM alert_history ORDER BY timestamp DESC LIMIT 10"
	rows, err = d.db.Query(recentAlertsQuery)
	if err != nil {
		d.logger.Errorf("获取最近告警失败: %v", err)
		return nil, err
	}
	defer rows.Close()

	var recentAlerts []types.AlertHistory
	for rows.Next() {
		var alert types.AlertHistory
		if err := rows.Scan(&alert.ID, &alert.AlertID, &alert.RuleName, &alert.Level, &alert.Message, &alert.Timestamp, &alert.Data, &alert.Count, &alert.Matches, &alert.CreatedAt); err != nil {
			d.logger.Errorf("扫描最近告警失败: %v", err)
			continue
		}
		recentAlerts = append(recentAlerts, alert)
	}
	stats.RecentAlerts = recentAlerts

	return stats, nil
}

// GetAlertsByRule 从数据库获取指定规则的告警历史
func (d *Database) GetAlertsByRule(ruleName string, limit int) ([]types.AlertHistory, error) {
	query := "SELECT * FROM alert_history WHERE rule_name = ? ORDER BY timestamp DESC LIMIT ?"
	rows, err := d.db.Query(query, ruleName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []types.AlertHistory
	for rows.Next() {
		var alert types.AlertHistory
		if err := rows.Scan(&alert.ID, &alert.AlertID, &alert.RuleName, &alert.Level, &alert.Message, &alert.Timestamp, &alert.Data, &alert.Count, &alert.Matches, &alert.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// GetAlertsByLevel 从数据库获取指定级别的告警历史
func (d *Database) GetAlertsByLevel(level string, limit int) ([]types.AlertHistory, error) {
	query := "SELECT * FROM alert_history WHERE level = ? ORDER BY timestamp DESC LIMIT ?"
	rows, err := d.db.Query(query, level, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []types.AlertHistory
	for rows.Next() {
		var alert types.AlertHistory
		if err := rows.Scan(&alert.ID, &alert.AlertID, &alert.RuleName, &alert.Level, &alert.Message, &alert.Timestamp, &alert.Data, &alert.Count, &alert.Matches, &alert.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

// GetAlertsPaged 分页查询（可选：按小时范围筛选）
func (d *Database) GetAlertsPaged(hours, page, pageSize int) ([]types.AlertHistory, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	var total int64
	baseWhere := ""
	args := []interface{}{}
	if hours > 0 {
		startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
		baseWhere = "WHERE timestamp >= ?"
		args = append(args, startTime)
		if err := d.db.QueryRow("SELECT COUNT(*) FROM alert_history "+baseWhere, args...).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		if err := d.db.QueryRow("SELECT COUNT(*) FROM alert_history").Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	query := "SELECT * FROM alert_history " + baseWhere + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []types.AlertHistory
	for rows.Next() {
		var alert types.AlertHistory
		if err := rows.Scan(&alert.ID, &alert.AlertID, &alert.RuleName, &alert.Level, &alert.Message, &alert.Timestamp, &alert.Data, &alert.Count, &alert.Matches, &alert.CreatedAt); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, total, nil
}

// (removed) GetAlertsAdvanced: 恢复旧版仅支持 GetAlertsPaged

// GetAlertByID 根据 alert_id 获取单条告警详情
func (d *Database) GetAlertByID(alertID string) (*types.AlertDetail, error) {
	query := "SELECT alert_id, rule_name, level, message, timestamp, data, count, matches FROM alert_history WHERE alert_id = ? LIMIT 1"

	var (
		id        string
		ruleName  string
		level     string
		message   string
		timestamp time.Time
		dataJSON  string
		count     int64
		matches   int64
	)

	err := d.db.QueryRow(query, alertID).Scan(&id, &ruleName, &level, &message, &timestamp, &dataJSON, &count, &matches)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var data map[string]interface{}
	if dataJSON != "" {
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			// 解析失败不致命，置空
			data = nil
		}
	}

	return &types.AlertDetail{
		ID:        id,
		RuleName:  ruleName,
		Level:     level,
		Message:   message,
		Timestamp: timestamp,
		Count:     count,
		Matches:   matches,
		Data:      data,
	}, nil
}

// SaveSession 保存用户会话
func (d *Database) SaveSession(sessionID, username, role string, expiresAt time.Time) error {
	var query string
	if d.dbType == "mysql" {
		query = `INSERT INTO user_sessions (session_id, username, role, expires_at)
                 VALUES (?, ?, ?, ?)
                 ON DUPLICATE KEY UPDATE
                   username=VALUES(username),
                   role=VALUES(role),
                   expires_at=VALUES(expires_at)`
	} else {
		query = `INSERT OR REPLACE INTO user_sessions (session_id, username, role, expires_at)
                 VALUES (?, ?, ?, ?)`
	}

	_, err := d.db.Exec(query, sessionID, username, role, expiresAt)
	if err != nil {
		return fmt.Errorf("保存用户会话失败: %w", err)
	}

	return nil
}

// AcquireRuleLock 尝试获取某条规则的执行锁
// 返回 true 表示成功获得租约
func (d *Database) AcquireRuleLock(ruleName, instanceID string, ttlSeconds int) (bool, error) {
	now := time.Now()
	// 先确保占位行存在
	if d.dbType == "mysql" {
		_, _ = d.db.Exec("INSERT IGNORE INTO rule_locks(rule_name, ttl_seconds) VALUES(?, ?)", ruleName, ttlSeconds)
		res, err := d.db.Exec(`UPDATE rule_locks 
            SET locked_by=?, locked_at=?
            WHERE rule_name=? AND (locked_at IS NULL OR locked_at <= DATE_SUB(?, INTERVAL ttl_seconds SECOND) OR locked_by=?)`,
			instanceID, now, ruleName, now, instanceID)
		if err != nil {
			return false, err
		}
		n, _ := res.RowsAffected()
		return n == 1, nil
	}
	// SQLite
	_, _ = d.db.Exec("INSERT OR IGNORE INTO rule_locks(rule_name, ttl_seconds) VALUES(?, ?)", ruleName, ttlSeconds)
	res, err := d.db.Exec(`UPDATE rule_locks 
        SET locked_by=?, locked_at=?
        WHERE rule_name=? AND (locked_at IS NULL OR locked_at <= datetime(?, '-' || ttl_seconds || ' seconds') OR locked_by=?)`,
		instanceID, now, ruleName, now, instanceID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// ReleaseRuleLock 释放规则锁
func (d *Database) ReleaseRuleLock(ruleName, instanceID string) error {
	if d.dbType == "mysql" {
		_, err := d.db.Exec(`UPDATE rule_locks SET locked_by='', locked_at = DATE_SUB(NOW(), INTERVAL ttl_seconds SECOND)
            WHERE rule_name=? AND locked_by=?`, ruleName, instanceID)
		return err
	}
	_, err := d.db.Exec(`UPDATE rule_locks SET locked_by='', locked_at = datetime('now','-1 second')
        WHERE rule_name=? AND locked_by=?`, ruleName, instanceID)
	return err
}

// GetSession 获取用户会话
func (d *Database) GetSession(sessionID string) (*types.User, error) {
	query := `
	SELECT username, role, expires_at 
	FROM user_sessions 
	WHERE session_id = ? AND expires_at > ?`

	var username, role string
	var expiresAt time.Time

	err := d.db.QueryRow(query, sessionID, time.Now()).Scan(&username, &role, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 会话不存在或已过期
		}
		return nil, fmt.Errorf("查询用户会话失败: %w", err)
	}

	return &types.User{
		Username: username,
		Role:     role,
	}, nil
}

// DeleteSession 删除用户会话
func (d *Database) DeleteSession(sessionID string) error {
	query := `DELETE FROM user_sessions WHERE session_id = ?`

	_, err := d.db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("删除用户会话失败: %w", err)
	}

	return nil
}

// ShouldSendAndTouch 发送前去重：
// - 以 (rule_name, level, message_hash) 作为去重签名（可加上聚合键等）
// - 若在 TTL 内已发送，则返回 false
// - 否则更新/插入最近发送时间并返回 true
func (d *Database) ShouldSendAndTouch(ruleName, level, message string, ttlSeconds int) (bool, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	// 计算消息哈希（避免长文本索引）
	h := sha1.Sum([]byte(message))
	messageHash := fmt.Sprintf("%x", h[:])
	dedupeKey := fmt.Sprintf("%s|%s|%s", ruleName, level, messageHash)

	now := time.Now()
	// MySQL 与 SQLite 写法分支
	if d.dbType == "mysql" {
		// 占位
		_, _ = d.db.Exec("INSERT IGNORE INTO alert_dedupe(dedupe_key, alert_id, rule_name, level, message_hash, last_sent, ttl_seconds) VALUES(?, '', ?, ?, ?, DATE_SUB(?, INTERVAL ttl_seconds SECOND), ?)", dedupeKey, ruleName, level, messageHash, now, ttlSeconds)
		// 检查是否过期
		var lastSent time.Time
		err := d.db.QueryRow("SELECT last_sent FROM alert_dedupe WHERE dedupe_key=?", dedupeKey).Scan(&lastSent)
		if err != nil && err != sql.ErrNoRows {
			return false, err
		}
		// 若在 TTL 内，拒绝发送
		if !lastSent.IsZero() && lastSent.After(now.Add(-time.Duration(ttlSeconds)*time.Second)) {
			return false, nil
		}
		// 更新为现在
		_, err = d.db.Exec("UPDATE alert_dedupe SET last_sent=?, ttl_seconds=? WHERE dedupe_key=?", now, ttlSeconds, dedupeKey)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	// SQLite
	_, _ = d.db.Exec("INSERT OR IGNORE INTO alert_dedupe(dedupe_key, alert_id, rule_name, level, message_hash, last_sent, ttl_seconds) VALUES(?, '', ?, ?, ?, datetime(?, '-' || ttl_seconds || ' seconds'), ?)", dedupeKey, ruleName, level, messageHash, now, ttlSeconds)
	var lastSentStr string
	err := d.db.QueryRow("SELECT last_sent FROM alert_dedupe WHERE dedupe_key=?", dedupeKey).Scan(&lastSentStr)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	var lastSent time.Time
	if lastSentStr != "" {
		// SQLite parse
		lst, perr := time.Parse("2006-01-02 15:04:05", lastSentStr)
		if perr == nil {
			lastSent = lst
		}
	}
	if !lastSent.IsZero() && lastSent.After(now.Add(-time.Duration(ttlSeconds)*time.Second)) {
		return false, nil
	}
	_, err = d.db.Exec("UPDATE alert_dedupe SET last_sent=?, ttl_seconds=? WHERE dedupe_key=?", now, ttlSeconds, dedupeKey)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CleanExpiredSessions 清理过期会话
func (d *Database) CleanExpiredSessions() error {
	query := `DELETE FROM user_sessions WHERE expires_at <= ?`

	_, err := d.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("清理过期会话失败: %w", err)
	}

	return nil
}
