package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"elasticsearch-alert/internal/config"
)

type Notifier interface {
	Name() string
	Send(ctx context.Context, title, text string) error
}

func BuildNotifiers(cfg config.Notifications) []Notifier {
	var notifiers []Notifier
	notifiers = append(notifiers, &ConsoleNotifier{})
	if cfg.Webhook.URL != "" {
		notifiers = append(notifiers, &WebhookNotifier{
			URL:     cfg.Webhook.URL,
			Headers: cfg.Webhook.Headers,
			Timeout: parseDurationDefault(cfg.Webhook.Timeout, 5*time.Second),
		})
	}
	if cfg.Feishu.Webhook != "" {
		notifiers = append(notifiers, &FeishuNotifier{
			Webhook:      cfg.Feishu.Webhook,
			EnableAtAll:  cfg.Feishu.EnableAtAll,
			Timeout:      parseDurationDefault(cfg.Feishu.Timeout, 5*time.Second),
			TitlePrefix:  cfg.Feishu.TitlePrefix,
			ContentIntro: cfg.Feishu.ContentIntro,
		})
	}
	if cfg.DingTalk.Webhook != "" {
		notifiers = append(notifiers, &DingTalkNotifier{
			Webhook:     cfg.DingTalk.Webhook,
			Secret:      cfg.DingTalk.Secret,
			EnableAtAll: cfg.DingTalk.EnableAtAll,
			Timeout:     parseDurationDefault(cfg.DingTalk.Timeout, 5*time.Second),
		})
	}
	if cfg.WeChat.Webhook != "" {
		notifiers = append(notifiers, &WeChatNotifier{
			Webhook: cfg.WeChat.Webhook,
			Timeout: parseDurationDefault(cfg.WeChat.Timeout, 5*time.Second),
		})
	}
	if cfg.Email.Host != "" && cfg.Email.From != "" && len(cfg.Email.To) > 0 {
		notifiers = append(notifiers, &EmailNotifier{
			Host:          cfg.Email.Host,
			Port:          cfg.Email.Port,
			Username:      cfg.Email.Username,
			Password:      cfg.Email.Password,
			From:          cfg.Email.From,
			To:            cfg.Email.To,
			UseTLS:        cfg.Email.UseTLS,
			TLSSkipVerify: cfg.Email.TLSSkipVerify,
			SubjectPrefix: cfg.Email.SubjectPrefix,
			Timeout:       parseDurationDefault(cfg.Email.Timeout, 10*time.Second),
		})
	}
	return notifiers
}

// Console
type ConsoleNotifier struct{}

func (c *ConsoleNotifier) Name() string { return "console" }
func (c *ConsoleNotifier) Send(ctx context.Context, title, text string) error {
	log.Printf("[ALERT][console] %s\n%s", title, text)
	return nil
}

// Webhook
type WebhookNotifier struct {
	URL     string
	Headers map[string]string
	Timeout time.Duration
}

func (w *WebhookNotifier) Name() string { return "webhook" }
func (w *WebhookNotifier) Send(ctx context.Context, title, text string) error {
	body := map[string]any{
		"title":   title,
		"message": text,
		"ts":      time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: w.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status=%d", resp.StatusCode)
	}
	return nil
}

func parseDurationDefault(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}


