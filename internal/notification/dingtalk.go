package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type DingTalkNotifier struct {
	Webhook     string
	Secret      string
	EnableAtAll bool
	Timeout     time.Duration
}

func (d *DingTalkNotifier) Name() string { return "dingtalk" }

func (d *DingTalkNotifier) Send(ctx context.Context, title, text string) error {
	// 参考 opensearch-alert-main 的 Markdown 模板，增加 Emoji 与标签
	content := fmt.Sprintf("**🚨 Elasticsearch 日志告警**\n\n"+
		"🏷️ **规则/标题：** %s\n\n"+
		"📝 **详情：**\n%s",
		title, text)

	// 钉钉 Markdown 中手动追加 @所有人 提示
	if d.EnableAtAll {
		content += "\n\n@所有人"
	}

	webhookURL := d.Webhook
	if d.Secret != "" {
		webhookURL = d.addSign(webhookURL, d.Secret)
	}

	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "Elasticsearch 日志告警",
			"text":  content,
		},
		"at": map[string]any{
			"isAtAll": d.EnableAtAll,
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: d.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk webhook status=%d body=%s", resp.StatusCode, string(body))
	}

	// 钉钉即使失败也会返回 200，通过 errcode 判断是否成功
	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &res); err == nil {
		if res.ErrCode != 0 {
			return fmt.Errorf("dingtalk webhook errcode=%d errmsg=%s body=%s", res.ErrCode, res.ErrMsg, string(body))
		}
	}
	return nil
}

// addSign 按钉钉官方文档对 webhook 进行加签
func (d *DingTalkNotifier) addSign(webhookURL, secret string) string {
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	stringToSign := timestamp + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	u, err := url.Parse(webhookURL)
	if err != nil {
		return webhookURL
	}
	q := u.Query()
	q.Set("timestamp", timestamp)
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	return u.String()
}
