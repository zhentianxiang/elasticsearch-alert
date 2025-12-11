package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WeChatNotifier struct {
	Webhook string
	Timeout time.Duration
}

func (w *WeChatNotifier) Name() string { return "wechat" }

func (w *WeChatNotifier) Send(ctx context.Context, title, text string) error {
	// 企业微信使用 Markdown，可以在标题前增加 Emoji 提示
	content := fmt.Sprintf("**🚨 %s**\n%s", title, text)
	// 在底部追加 @所有人 提示（企业微信 markdown 类型不支持真正的 mentioned_list，这里仅作视觉提醒）
	content += "\n\n@所有人"
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Webhook, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: w.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("wechat webhook status=%d", resp.StatusCode)
	}
	return nil
}
