package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"opensearch-alert/internal/alert"
	"opensearch-alert/internal/config"
	"opensearch-alert/internal/database"
	"opensearch-alert/internal/notification"
	"opensearch-alert/internal/opensearch"
	"opensearch-alert/internal/web"
	"opensearch-alert/pkg/types"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "./configs/config.yaml", "配置文件路径")
	rulesPath  = flag.String("rules", "./configs/rules", "规则文件目录")
)

func main() {
	flag.Parse()

	// 检测用户是否显式传入了 -rules 参数
	rulesFlagProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "rules" {
			rulesFlagProvided = true
		}
	})

	// 自动检测运行环境并设置默认路径
	if *configPath == "./configs/config.yaml" {
		// 检查容器环境
		if _, err := os.Stat("/app/config/config.yaml"); err == nil {
			*configPath = "/app/config/config.yaml"
			if !rulesFlagProvided {
				*rulesPath = "/app/config/rules"
			}
		} else {
			// 检查当前目录下的配置文件
			if _, err := os.Stat("./configs/config.yaml"); err != nil {
				// 如果当前目录没有，尝试从可执行文件目录查找
				exePath, _ := os.Executable()
				exeDir := filepath.Dir(exePath)
				configFile := filepath.Join(exeDir, "configs", "config.yaml")
				if _, err := os.Stat(configFile); err == nil {
					*configPath = configFile
					if !rulesFlagProvided {
						*rulesPath = filepath.Join(exeDir, "configs", "rules")
					}
				} else {
					// 最后尝试从可执行文件目录的上级目录查找
					parentDir := filepath.Dir(exeDir)
					configFile = filepath.Join(parentDir, "configs", "config.yaml")
					if _, err := os.Stat(configFile); err == nil {
						*configPath = configFile
						if !rulesFlagProvided {
							*rulesPath = filepath.Join(parentDir, "configs", "rules")
						}
					}
				}
			}
		}
	}

	// 先加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		// 如果配置加载失败，使用默认日志配置
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
		logger.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 设置日志级别
	if level, err := logrus.ParseLevel(cfg.Logging.Level); err == nil {
		logger.SetLevel(level)
	}

	// 设置日志文件输出（同时输出到终端和文件）
	if cfg.Logging.File != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.Logging.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Warnf("创建日志目录失败: %v", err)
		} else {
			// 创建文件输出
			file, err := os.OpenFile(cfg.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				logger.Warnf("打开日志文件失败: %v", err)
			} else {
				// 使用 MultiWriter 同时输出到终端和文件
				multiWriter := io.MultiWriter(os.Stdout, file)
				logger.SetOutput(multiWriter)
				logger.Info("日志将同时输出到终端和文件: " + cfg.Logging.File)
			}
		}
	}

	logger.Info("🚀 启动 OpenSearch 告警工具...")
	logger.Infof("📁 配置文件: %s", *configPath)
	logger.Infof("📁 规则目录(参数): %s", *rulesPath)
	// 若命令行未显式指定，优先使用配置中的 rules_folder
	if !rulesFlagProvided && cfg.Rules.RulesFolder != "" {
		*rulesPath = cfg.Rules.RulesFolder
	}
	// 将最终生效的规则目录同步到内存配置，确保 Web 管理页与引擎一致
	if cfg.Rules.RulesFolder != *rulesPath {
		cfg.Rules.RulesFolder = *rulesPath
	}
	logger.Infof("📁 规则目录(生效): %s", *rulesPath)
	logger.Infof("🔧 日志级别: %s", cfg.Logging.Level)
	if cfg.Logging.File != "" {
		logger.Infof("📝 日志文件: %s", cfg.Logging.File)
	}

	// 显示 OpenSearch 连接信息
	logger.Infof("🔗 OpenSearch 连接: %s://%s:%d", cfg.OpenSearch.Protocol, cfg.OpenSearch.Host, cfg.OpenSearch.Port)
	logger.Infof("👤 用户名: %s", cfg.OpenSearch.Username)
	logger.Infof("⏱️  超时时间: %d秒", cfg.OpenSearch.Timeout)

	// 创建 OpenSearch 客户端
	logger.Info("🔧 创建 OpenSearch 客户端...")
	opensearchClient := opensearch.NewClient(cfg.OpenSearch)

	// 测试 OpenSearch 连接
	logger.Info("🔍 测试 OpenSearch 连接...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := opensearchClient.TestConnection(ctx); err != nil {
		logger.Errorf("❌ OpenSearch 连接测试失败: %v", err)
		logger.Fatal("OpenSearch 连接失败，程序退出")
	} else {
		logger.Info("✅ OpenSearch 连接测试成功")
	}

	// 创建数据库连接
	logger.Info("🔧 创建数据库连接...")
	db, err := database.NewDatabase(cfg.Database, logger)
	if err != nil {
		logger.Fatalf("❌ 创建数据库连接失败: %v", err)
	}
	defer db.Close()

	// 先加载规则并完成引擎初始化再创建通知器/发送测试

	// 在加载前，先将内置规则引导写入目标目录（不覆盖已有文件）
	if written, bootErr := config.BootstrapEmbeddedRules(*rulesPath, false, logger); bootErr != nil {
		logger.Warnf("引导内置规则失败: %v", bootErr)
	} else if written > 0 {
		logger.Infof("🧩 已生成 %d 个内置规则", written)
	}

	// 加载告警规则
	logger.Info("📋 加载告警规则...")
	rules, err := config.LoadRules(*rulesPath)
	if err != nil {
		logger.Fatalf("❌ 加载告警规则失败: %v", err)
	}

	// 使用配置默认值回填缺失的 timeframe 与 threshold
	for i := range rules {
		if rules[i].Timeframe == 0 {
			rules[i].Timeframe = cfg.Rules.DefaultTimeframe
		}
		if rules[i].Threshold == 0 {
			rules[i].Threshold = cfg.Rules.DefaultThreshold
		}
	}

	if len(rules) == 0 {
		logger.Warn("⚠️  没有找到启用的告警规则")
	} else {
		logger.Infof("✅ 成功加载 %d 个告警规则", len(rules))
		for i, rule := range rules {
			logger.Infof("  %d. %s (%s) - 索引: %s", i+1, rule.Name, rule.Type, rule.Index)
		}
	}

	// 创建通知器（在规则无误后再初始化通知渠道）
	logger.Info("🔧 创建通知器...")
	notifier := notification.NewNotifier(cfg, logger)

	// 显示启用的通知渠道
	enabledChannels := []string{}
	if cfg.Notifications.Email.Enabled {
		enabledChannels = append(enabledChannels, "邮件")
	}
	if cfg.Notifications.DingTalk.Enabled {
		enabledChannels = append(enabledChannels, "钉钉")
	}
	if cfg.Notifications.WeChat.Enabled {
		enabledChannels = append(enabledChannels, "企业微信")
	}
	if cfg.Notifications.Feishu.Enabled {
		enabledChannels = append(enabledChannels, "飞书")
	}
	if len(enabledChannels) > 0 {
		logger.Infof("📢 启用的通知渠道: %v", enabledChannels)
	} else {
		logger.Warn("⚠️  没有启用任何通知渠道")
	}

	// 创建告警引擎
	logger.Info("🔧 创建告警引擎...")
	alertEngine := alert.NewEngine(cfg, opensearchClient, notifier, db, logger)
	alertEngine.LoadRules(rules)

	// 显示告警引擎配置
	logger.Infof("⚙️  告警引擎配置:")
	logger.Infof("  - 检查间隔: %d秒", cfg.AlertEngine.RunInterval)
	logger.Infof("  - 缓冲时间: %d秒", cfg.AlertEngine.BufferTime)
	logger.Infof("  - 最大并发规则数: %d", cfg.AlertEngine.MaxRunningRules)
	logger.Infof("  - 状态索引: %s", cfg.AlertEngine.WritebackIndex)
	logger.Infof("  - 告警保留时间: %d秒", cfg.AlertEngine.AlertTimeLimit)

	// 启动告警引擎
	logger.Info("🚀 启动告警引擎...")
	if err := alertEngine.Start(); err != nil {
		logger.Fatalf("❌ 启动告警引擎失败: %v", err)
	}

	// 服务启动测试通知（放到最后）
	if len(enabledChannels) > 0 {
		logger.Info("🎉 服务启动成功！发送启动测试通知...")
		testAlert := &types.Alert{
			ID:        fmt.Sprintf("startup-test-%d", time.Now().Unix()),
			RuleName:  "服务启动测试",
			Level:     "Info",
			Message:   "🚀 OpenSearch 告警工具启动成功！",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"test": true},
			Count:     1,
			Matches:   1,
		}
		if err := notifier.SendAlert(testAlert); err != nil {
			logger.Errorf("❌ 启动测试通知发送失败: %v", err)
		} else {
			logger.Info("✅ 启动测试通知发送完成")
		}
	}

	// 启动 Web 服务器
	var webServer *web.Server
	if cfg.Web.Enabled {
		logger.Info("🌐 启动 Web 服务器...")
		webServer = web.NewServer(cfg, db, notifier, alertEngine, logger)

		go func() {
			if err := webServer.Start(); err != nil {
				logger.Errorf("❌ Web 服务器启动失败: %v", err)
			}
		}()

		logger.Infof("🌐 Web 服务器已启动: http://%s:%d", cfg.Web.Host, cfg.Web.Port)
		logger.Infof("📊 Dashboard: http://%s:%d/dashboard", cfg.Web.Host, cfg.Web.Port)
		logger.Infof("🔐 登录页面: http://%s:%d/login", cfg.Web.Host, cfg.Web.Port)
	}

	logger.Info("🎉 OpenSearch 告警工具已成功启动！")
	logger.Info("💡 使用 Ctrl+C 停止程序")

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	<-sigChan
	logger.Info("收到退出信号，正在关闭...")

	// 停止告警引擎
	alertEngine.Stop()

	logger.Info("OpenSearch 告警工具已关闭")
}
