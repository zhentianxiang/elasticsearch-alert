package notification

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"opensearch-alert/pkg/types"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// DingTalkNotifier 钉钉通知器
type DingTalkNotifier struct {
	config *types.DingTalkConfig
	logger *logrus.Logger
}

// NewDingTalkNotifier 创建钉钉通知器
func NewDingTalkNotifier(config *types.DingTalkConfig, logger *logrus.Logger) *DingTalkNotifier {
	return &DingTalkNotifier{
		config: config,
		logger: logger,
	}
}

// IsEnabled 检查是否启用
func (d *DingTalkNotifier) IsEnabled() bool {
	return d.config.Enabled
}

// Send 发送钉钉消息
func (d *DingTalkNotifier) Send(alert *types.Alert) error {
	if !d.IsEnabled() {
		return nil
	}

	// 构建消息
	message := d.buildDingTalkMessage(alert)

	// 发送请求
	webhookURL := d.config.WebhookURL
	if d.config.Secret != "" {
		webhookURL = d.addSign(webhookURL, d.config.Secret)
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送钉钉消息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("钉钉消息发送失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应内容用于调试
	body, _ := io.ReadAll(resp.Body)
	d.logger.Debugf("钉钉消息发送成功，响应: %s", string(body))

	d.logger.Infof("钉钉告警已发送: %s", alert.RuleName)
	return nil
}

// buildDingTalkMessage 构建钉钉消息
func (d *DingTalkNotifier) buildDingTalkMessage(alert *types.Alert) map[string]interface{} {
	// 构建@用户信息
	at := map[string]interface{}{
		"atMobiles": d.config.AtMobiles,
		"isAtAll":   d.config.AtAll,
	}

	// 构建@文本 - 只有严重告警才@用户
	atText := ""
	if d.shouldAtUser(alert.Level) {
		// 如果配置了@所有人，或者没有配置具体用户，则@所有人
		if d.config.AtAll || len(d.config.AtMobiles) == 0 {
			atText = "@所有人 "
		} else if len(d.config.AtMobiles) > 0 {
			// 如果有具体用户配置，则@具体用户
			for _, mobile := range d.config.AtMobiles {
				atText += fmt.Sprintf("@%s ", mobile)
			}
		}
	}

	// 构建Markdown内容（表情+标签），并追加详情
	markdown := fmt.Sprintf("**%s KubeSphere-OpenSearch 告警通知**\n\n"+
		"🏷️ **规则名称:** %s\n"+
		"%s **告警级别:** %s\n"+
		"🕒 **触发时间:** %s\n"+
		"📈 **匹配数量:** %d\n\n"+
		"📝 **详情:**\n%s",
		d.getLevelEmoji(alert.Level),
		alert.RuleName,
		d.getLevelEmoji(alert.Level), alert.Level,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Count,
		d.formatMessageContent(alert.Message))

	// 处理消息内容，确保在钉钉中正确显示
	// 钉钉 Markdown 需要在换行符前后各添加两个空格才能正确换行
	// 将消息内容中的换行符替换为 "  \n  " 格式
	markdown = strings.ReplaceAll(markdown, "\n", "  \n  ")

	// 添加@信息
	if atText != "" {
		markdown += "\n\n" + atText
	}

	// 构建消息体
	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "KubeSphere-OpenSearch 告警通知",
			"text":  markdown,
		},
		"at": at,
	}

	return message
}

// getLevelEmoji 不同级别对应的图标
func (d *DingTalkNotifier) getLevelEmoji(level string) string {
	switch level {
	case "Critical":
		return "🚨"
	case "High":
		return "🚩"
	case "Medium":
		return "🔔"
	case "Low", "Info":
		return "ℹ️"
	default:
		return "🔔"
	}
}

// formatMessageContent 钉钉Markdown兼容处理：移除分隔线、代码块标记并压缩空行
func (d *DingTalkNotifier) formatMessageContent(message string) string {
	formatted := message

	// 去掉代码块围栏，保留内容
	formatted = strings.ReplaceAll(formatted, "```", "")

	// 移除 '---' 和仅由横线组成的整行
	formatted = strings.ReplaceAll(formatted, "---", "")
	hyphenDivider := regexp.MustCompile(`(?m)^\s*-{6,}\s*$`)
	formatted = hyphenDivider.ReplaceAllString(formatted, "")

	// 压缩多余空行到最多两个
	multiEmpty := regexp.MustCompile(`\n{3,}`)
	formatted = multiEmpty.ReplaceAllString(formatted, "\n\n")

	return strings.TrimSpace(formatted)
}

// shouldAtUser 判断是否应该@用户
func (d *DingTalkNotifier) shouldAtUser(level string) bool {
	// 只有严重和高优先级告警才@用户
	switch level {
	case "Critical", "High":
		return true
	case "Medium", "Low", "Info":
		return false
	default:
		return false
	}
}

// addSign 添加签名
func (d *DingTalkNotifier) addSign(webhookURL, secret string) string {
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	stringToSign := timestamp + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	u, _ := url.Parse(webhookURL)
	q := u.Query()
	q.Set("timestamp", timestamp)
	q.Set("sign", sign)
	u.RawQuery = q.Encode()

	return u.String()
}
