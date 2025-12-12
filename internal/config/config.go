package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Scheduler     SchedulerConfig     `yaml:"scheduler"`
	Rules         RulesConfig         `yaml:"rules"`
	Notifications Notifications       `yaml:"notifications"`
	Web           WebConfig           `yaml:"web"`
	Logging       LoggingConfig       `yaml:"logging"`
}

type ElasticsearchConfig struct {
	Addresses        []string `yaml:"addresses"`
	Username         string   `yaml:"username"`
	Password         string   `yaml:"password"`
	CloudID          string   `yaml:"cloudId"`
	APIKey           string   `yaml:"apiKey"`
	TLSSkipVerify    bool     `yaml:"tlsSkipVerify"`
	RequestTimeout   string   `yaml:"requestTimeout"`
	Provider         string   `yaml:"provider"` // elasticsearch | opensearch
	SkipProductCheck bool     `yaml:"skipProductCheck"`
}

func (e ElasticsearchConfig) GetRequestTimeout() time.Duration {
	if e.RequestTimeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(e.RequestTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

type SchedulerConfig struct {
	Timezone string `yaml:"timezone"`
}

type RulesConfig struct {
	Directory          string `yaml:"directory"`
	SampleSize         int    `yaml:"sampleSize"`
	DefaultQuietPeriod string `yaml:"defaultQuietPeriod"`
}

func (r RulesConfig) GetDefaultQuietPeriod() time.Duration {
	if r.DefaultQuietPeriod == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(r.DefaultQuietPeriod)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

type Notifications struct {
	Webhook  WebhookConfig  `yaml:"webhook"`
	Feishu   FeishuConfig   `yaml:"feishu"`
	DingTalk DingTalkConfig `yaml:"dingtalk"`
	WeChat   WeChatConfig   `yaml:"wechat"`
	Email    EmailConfig    `yaml:"email"`
}

// WebConfig 控制内置 HTTP Web 服务（查看单条日志详情）
type WebConfig struct {
	Enabled bool   `yaml:"enabled"` // 是否开启 Web 服务
	Listen  string `yaml:"listen"`  // 监听地址，如 ":8080"
	BaseURL string `yaml:"baseURL"` // 对外访问的基础地址，用于在通知中生成跳转链接，如 "http://alert.example.com:8080"
}

// LoggingConfig 控制日志级别
type LoggingConfig struct {
	// Level 支持 INFO / DEBUG（大小写不敏感），默认 INFO。
	Level string `yaml:"level"`
}

type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Timeout string            `yaml:"timeout"`
}

type FeishuConfig struct {
	Webhook      string `yaml:"webhook"`
	EnableAtAll  bool   `yaml:"enableAtAll"`
	Timeout      string `yaml:"timeout"`
	TitlePrefix  string `yaml:"titlePrefix"`
	ContentIntro string `yaml:"contentIntro"`
}

type DingTalkConfig struct {
	Webhook     string `yaml:"webhook"`
	Secret      string `yaml:"secret"`
	EnableAtAll bool   `yaml:"enableAtAll"`
	Timeout     string `yaml:"timeout"`
}

type WeChatConfig struct {
	Webhook string `yaml:"webhook"`
	Timeout string `yaml:"timeout"`
}

type EmailConfig struct {
	Host          string   `yaml:"host"`
	Port          int      `yaml:"port"`
	Username      string   `yaml:"username"`
	Password      string   `yaml:"password"`
	From          string   `yaml:"from"`
	To            []string `yaml:"to"`
	UseTLS        bool     `yaml:"useTLS"`
	TLSSkipVerify bool     `yaml:"tlsSkipVerify"`
	SubjectPrefix string   `yaml:"subjectPrefix"`
	Timeout       string   `yaml:"timeout"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if cfg.Rules.SampleSize <= 0 {
		cfg.Rules.SampleSize = 3
	}
	if cfg.Scheduler.Timezone == "" {
		cfg.Scheduler.Timezone = "Asia/Shanghai"
	}
	if cfg.Elasticsearch.Provider == "" {
		cfg.Elasticsearch.Provider = "elasticsearch"
	}
	if cfg.Web.Listen == "" {
		cfg.Web.Listen = ":8080"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "INFO"
	}
	// 环境变量 LOG_LEVEL 优先级高于配置文件
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	return &cfg, nil
}
