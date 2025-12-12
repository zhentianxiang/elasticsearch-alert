package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"elasticsearch-alert/internal/alert"
	"elasticsearch-alert/internal/config"
	"elasticsearch-alert/internal/elasticsearch"
	"elasticsearch-alert/internal/logging"
	"elasticsearch-alert/internal/notification"
	"elasticsearch-alert/internal/web"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志级别（INFO / DEBUG），优先使用环境变量 LOG_LEVEL
	logging.Init(cfg.Logging.Level)
	logging.Infof("日志级别已设置为: %s", cfg.Logging.Level)

	// Set environment variable for skipping Elasticsearch product check
	if cfg.Elasticsearch.SkipProductCheck {
		if err := os.Setenv("ELASTIC_CLIENT_SKIP_PRODUCT_CHECK", "true"); err != nil {
			logging.Errorf("警告: 无法设置环境变量 ELASTIC_CLIENT_SKIP_PRODUCT_CHECK: %v", err)
		}
	}

	esClient, err := elasticsearch.NewClient(cfg.Elasticsearch)
	if err != nil {
		log.Fatalf("初始化 Elasticsearch 客户端失败: %v", err)
	}
	logging.Infof("Elasticsearch 客户端初始化完成，地址=%v", cfg.Elasticsearch.Addresses)

	notifiers := notification.BuildNotifiers(cfg.Notifications)

	engine, err := alert.NewEngine(cfg, esClient, notifiers)
	if err != nil {
		log.Fatalf("初始化告警引擎失败: %v", err)
	}
	logging.Infof("告警引擎初始化完成，规则数量=%d", len(engine.Rules()))

	// 启动内置 Web 服务，用于查看单条日志详情
	if cfg.Web.Enabled {
		webServer := web.NewServer(cfg, esClient)
		go func() {
			if err := webServer.Start(); err != nil {
				logging.Errorf("Web 服务异常退出: %v", err)
			}
		}()
	}

	if err := engine.Start(); err != nil {
		log.Fatalf("启动告警引擎失败: %v", err)
	}
	logging.Infof("elasticsearch-alert 已启动，时区=%s", cfg.Scheduler.Timezone)

	// graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	engine.Stop()
	logging.Infof("elasticsearch-alert 已停止")
}
