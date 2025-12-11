package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// 飞书简单文本卡片通知（支持 @all）
type FeishuNotifier struct {
	Webhook      string
	EnableAtAll  bool
	Timeout      time.Duration
	TitlePrefix  string
	ContentIntro string
}

func (f *FeishuNotifier) Name() string { return "feishu" }

func (f *FeishuNotifier) Send(ctx context.Context, title, text string) error {
	displayTitle := title
	if f.TitlePrefix != "" {
		displayTitle = f.TitlePrefix + " " + title
	}
	// 为飞书标题增加统一的告警 Emoji 前缀
	displayTitle = "🚨 " + displayTitle
	if f.ContentIntro != "" {
		text = f.ContentIntro + "\n\n" + text
	}
	// @所有人放在消息最底部，更符合阅读习惯
	if f.EnableAtAll {
		text = text + "\n\n<at id=all></at>"
	}

	// 使用交互式卡片样式，结构化展示内容
	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"header": map[string]any{
				"title": map[string]any{
					"tag":     "plain_text",
					"content": displayTitle,
				},
				// 统一使用红色模板，高优先级视觉效果更好
				"template": "red",
			},
			"elements": []map[string]any{
				{
					"tag": "div",
					"text": map[string]any{
						"tag":     "lark_md",
						"content": text,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.Webhook, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: f.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("feishu webhook status=%d", resp.StatusCode)
	}
	return nil
}
