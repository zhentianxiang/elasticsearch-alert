package config

import (
	"fmt"
	"opensearch-alert/pkg/types"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*types.Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	return &config, nil
}

// LoadRules 加载告警规则
func LoadRules(rulesFolder string) ([]types.AlertRule, error) {
	var rules []types.AlertRule

	// 创建日志器
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	// 与主程序保持一致的时间格式，避免出现 INFO[0000] 相对时间样式
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	logger.Debugf("开始加载规则文件，目录: %s", rulesFolder)

	files, err := filepath.Glob(filepath.Join(rulesFolder, "*.yaml"))
	if err != nil {
		logger.Errorf("读取规则文件失败: %v", err)
		return nil, fmt.Errorf("读取规则文件失败: %w", err)
	}

	logger.Debugf("找到 %d 个规则文件", len(files))

	for _, file := range files {
		logger.Debugf("加载规则文件: %s", file)

		data, err := os.ReadFile(file)
		if err != nil {
			logger.Errorf("读取规则文件 %s 失败: %v", file, err)
			return nil, fmt.Errorf("读取规则文件 %s 失败: %w", file, err)
		}

		var rule types.AlertRule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			logger.Errorf("解析规则文件 %s 失败: %v", file, err)
			return nil, fmt.Errorf("解析规则文件 %s 失败: %w", file, err)
		}

		// 只加载启用的规则
		if rule.Enabled {
			logger.Debugf("加载启用规则: %s (级别: %s)", rule.Name, rule.Level)
			rules = append(rules, rule)
		} else {
			logger.Debugf("跳过禁用规则: %s", rule.Name)
		}
	}

	logger.Debugf("规则加载完成，共加载 %d 个启用规则", len(rules))
	return rules, nil
}

// setDefaults 设置默认值
func setDefaults(config *types.Config) {
	if config.AlertEngine.RunInterval == 0 {
		config.AlertEngine.RunInterval = 60
	}
	if config.AlertEngine.BufferTime == 0 {
		config.AlertEngine.BufferTime = 300
	}
	if config.AlertEngine.MaxRunningRules == 0 {
		config.AlertEngine.MaxRunningRules = 10
	}
	if config.AlertEngine.WritebackIndex == "" {
		config.AlertEngine.WritebackIndex = "opensearch_alert_status"
	}
	if config.AlertEngine.AlertTimeLimit == 0 {
		config.AlertEngine.AlertTimeLimit = 172800 // 2天
	}

	if config.AlertSuppression.RealertMinutes == 0 {
		config.AlertSuppression.RealertMinutes = 5
	}

	if config.Rules.DefaultTimeframe == 0 {
		config.Rules.DefaultTimeframe = 300
	}
	if config.Rules.DefaultThreshold == 0 {
		config.Rules.DefaultThreshold = 1
	}
	if config.Rules.RulesFolder == "" {
		config.Rules.RulesFolder = "configs/rules"
	}

	// Web 服务默认值
	if config.Web.Host == "" {
		config.Web.Host = "0.0.0.0"
	}
	if config.Web.Port == 0 {
		config.Web.Port = 8080
	}
	if config.Web.StaticPath == "" {
		config.Web.StaticPath = "web/static"
	}
	if config.Web.TemplatePath == "" {
		config.Web.TemplatePath = "web/templates"
	}
	if config.Web.SessionSecret == "" {
		config.Web.SessionSecret = "opensearch-alert-secret-key-2024"
	}

	// 数据库默认值
	if config.Database.Type == "" {
		config.Database.Type = "sqlite"
	}
	if config.Database.Path == "" {
		config.Database.Path = "data/opensearch-alert.db"
	}
	if config.Database.MaxConnections == 0 {
		config.Database.MaxConnections = 10
	}
	if config.Database.MaxIdleConnections == 0 {
		config.Database.MaxIdleConnections = 5
	}

	// 认证默认值
	if config.Auth.SessionTimeout == 0 {
		config.Auth.SessionTimeout = 3600
	}
}
