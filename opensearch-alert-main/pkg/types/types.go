package types

import (
	"time"
)

// Config 主配置结构
type Config struct {
	OpenSearch       OpenSearchConfig       `yaml:"opensearch"`
	AlertEngine      AlertEngineConfig      `yaml:"alert_engine"`
	AlertSuppression AlertSuppressionConfig `yaml:"alert_suppression"`
	Notifications    NotificationsConfig    `yaml:"notifications"`
	Logging          LoggingConfig          `yaml:"logging"`
	Web              WebConfig              `yaml:"web"`
	Database         DatabaseConfig         `yaml:"database"`
	Auth             AuthConfig             `yaml:"auth"`
	Rules            RulesConfig            `yaml:"rules"`
}

// OpenSearchConfig OpenSearch 连接配置
type OpenSearchConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Protocol    string `yaml:"protocol"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	VerifyCerts bool   `yaml:"verify_certs"`
	Timeout     int    `yaml:"timeout"`
}

// AlertEngineConfig 告警引擎配置
type AlertEngineConfig struct {
	RunInterval     int    `yaml:"run_interval"`
	BufferTime      int    `yaml:"buffer_time"`
	MaxRunningRules int    `yaml:"max_running_rules"`
	WritebackIndex  string `yaml:"writeback_index"`
	AlertTimeLimit  int    `yaml:"alert_time_limit"`
}

// AlertSuppressionConfig 告警抑制配置
type AlertSuppressionConfig struct {
	Enabled            bool                     `yaml:"enabled"`
	RealertMinutes     int                      `yaml:"realert_minutes"`
	ExponentialRealert ExponentialRealertConfig `yaml:"exponential_realert"`
}

// ExponentialRealertConfig 指数级告警间隔配置
type ExponentialRealertConfig struct {
	Enabled bool `yaml:"enabled"`
	Hours   int  `yaml:"hours"`
}

// NotificationsConfig 通知配置
type NotificationsConfig struct {
	Email    EmailConfig    `yaml:"email"`
	DingTalk DingTalkConfig `yaml:"dingtalk"`
	WeChat   WeChatConfig   `yaml:"wechat"`
	Feishu   FeishuConfig   `yaml:"feishu"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled    bool     `yaml:"enabled"`
	SMTPServer string   `yaml:"smtp_server"`
	SMTPPort   int      `yaml:"smtp_port"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	FromEmail  string   `yaml:"from_email"`
	ToEmails   []string `yaml:"to_emails"`
	UseTLS     bool     `yaml:"use_tls"`
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	Enabled    bool     `yaml:"enabled"`
	WebhookURL string   `yaml:"webhook_url"`
	Secret     string   `yaml:"secret"`
	AtMobiles  []string `yaml:"at_mobiles"`
	AtAll      bool     `yaml:"at_all"`
}

// WeChatConfig 企业微信配置
type WeChatConfig struct {
	Enabled             bool     `yaml:"enabled"`
	WebhookURL          string   `yaml:"webhook_url"`
	MentionedList       []string `yaml:"mentioned_list"`
	MentionedMobileList []string `yaml:"mentioned_mobile_list"`
	AtAll               bool     `yaml:"at_all"`
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled    bool     `yaml:"enabled"`
	WebhookURL string   `yaml:"webhook_url"`
	Secret     string   `yaml:"secret"`
	AtMobiles  []string `yaml:"at_mobiles"`
	AtAll      bool     `yaml:"at_all"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level       string `yaml:"level"`
	Format      string `yaml:"format"`
	File        string `yaml:"file"`
	MaxSize     string `yaml:"max_size"`
	BackupCount int    `yaml:"backup_count"`
}

// WebConfig Web 服务配置
type WebConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	StaticPath    string `yaml:"static_path"`
	TemplatePath  string `yaml:"template_path"`
	SessionSecret string `yaml:"session_secret"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type               string `yaml:"type"`
	Path               string `yaml:"path"`
	MaxConnections     int    `yaml:"max_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	// MySQL 配置（当 type=mysql 时生效）
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	Params   string `yaml:"params"` // 额外 DSN 参数, 例如 "tls=false&charset=utf8mb4"
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled        bool   `yaml:"enabled"`
	SessionTimeout int    `yaml:"session_timeout"`
	Users          []User `yaml:"users"`
}

// User 用户配置
type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`
}

// RulesConfig 规则配置
type RulesConfig struct {
	RulesFolder      string `yaml:"rules_folder"`
	DefaultTimeframe int    `yaml:"default_timeframe"`
	DefaultThreshold int    `yaml:"default_threshold"`
}

// AlertRule 告警规则结构
type AlertRule struct {
	Name          string                 `yaml:"name"`
	Type          string                 `yaml:"type"` // frequency, any, spike, flatline, change
	Index         string                 `yaml:"index"`
	Query         map[string]interface{} `yaml:"query"`
	Threshold     int                    `yaml:"threshold"`
	Timeframe     int                    `yaml:"timeframe"`
	QueryKey      []string               `yaml:"query_key"`
	Realert       int                    `yaml:"realert"`
	Alert         []string               `yaml:"alert"`
	AlertText     string                 `yaml:"alert_text"`
	AlertTextArgs []string               `yaml:"alert_text_args"`
	Level         string                 `yaml:"level"` // Critical, High, Medium, Low, Info
	Enabled       bool                   `yaml:"enabled"`
}

// Alert 告警结构
type Alert struct {
	ID        string                 `json:"id"`
	RuleName  string                 `json:"rule_name"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Count     int                    `json:"count"`
	Matches   int                    `json:"matches"`
}

// AlertStatus 告警状态
type AlertStatus struct {
	RuleName      string    `json:"rule_name"`
	LastAlert     time.Time `json:"last_alert"`
	AlertCount    int       `json:"alert_count"`
	Suppressed    bool      `json:"suppressed"`
	SuppressUntil time.Time `json:"suppress_until"`
}

// OpenSearchHit OpenSearch 查询结果
type OpenSearchHit struct {
	Index  string                 `json:"_index"`
	ID     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

// OpenSearchResponse OpenSearch 查询响应
type OpenSearchResponse struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		MaxScore float64         `json:"max_score"`
		Hits     []OpenSearchHit `json:"hits"`
	} `json:"hits"`
}

// AlertHistory 告警历史记录
type AlertHistory struct {
	ID        int64     `json:"-" db:"id"`
	AlertID   string    `json:"id" db:"alert_id"`
	RuleName  string    `json:"rule_name" db:"rule_name"`
	Level     string    `json:"level" db:"level"`
	Message   string    `json:"message" db:"message"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	Data      string    `json:"-" db:"data"` // JSON 字符串, 不暴露给前端
	Count     int64     `json:"count" db:"count"`
	Matches   int64     `json:"matches" db:"matches"`
	CreatedAt time.Time `json:"-" db:"created_at"`
}

// AlertDetail 告警详情（用于API返回，包含数据）
type AlertDetail struct {
	ID        string                 `json:"id"`
	RuleName  string                 `json:"rule_name"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Count     int64                  `json:"count"`
	Matches   int64                  `json:"matches"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// AlertStats 告警统计
type AlertStats struct {
	TotalAlerts  int64            `json:"total_alerts"`
	LevelStats   map[string]int64 `json:"level_stats"`
	RecentAlerts []AlertHistory   `json:"recent_alerts"`
	HourlyStats  []HourlyStat     `json:"hourly_stats"`
}

// HourlyStat 小时统计
type HourlyStat struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// DashboardData Dashboard 数据
type DashboardData struct {
	AlertStats AlertStats  `json:"alert_stats"`
	Rules      []AlertRule `json:"rules"`
	Config     *Config     `json:"config"`
	User       *User       `json:"user"`
}

// AlertQueryOptions 告警查询选项（用于前后端灵活筛选）
// (removed) AlertQueryOptions: 恢复旧版 API 仅支持 rule/level/hours

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    *User  `json:"user,omitempty"`
}
