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
	"elasticsearch-alert/internal/notification"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config error: %v", err)
	}

	// Set environment variable for skipping Elasticsearch product check
	if cfg.Elasticsearch.SkipProductCheck {
		if err := os.Setenv("ELASTIC_CLIENT_SKIP_PRODUCT_CHECK", "true"); err != nil {
			log.Printf("warning: failed to set ELASTIC_CLIENT_SKIP_PRODUCT_CHECK: %v", err)
		}
	}

	esClient, err := elasticsearch.NewClient(cfg.Elasticsearch)
	if err != nil {
		log.Fatalf("init elasticsearch client error: %v", err)
	}

	notifiers := notification.BuildNotifiers(cfg.Notifications)

	engine, err := alert.NewEngine(cfg, esClient, notifiers)
	if err != nil {
		log.Fatalf("init alert engine error: %v", err)
	}
	if err := engine.Start(); err != nil {
		log.Fatalf("start engine error: %v", err)
	}
	log.Printf("elasticsearch-alert is running, timezone=%s", cfg.Scheduler.Timezone)

	// graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	engine.Stop()
	log.Printf("elasticsearch-alert stopped")
}
