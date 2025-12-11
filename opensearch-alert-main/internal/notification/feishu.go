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
	"opensearch-alert/pkg/types"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// FeishuNotifier 飞书通知器
type FeishuNotifier struct {
	config *types.FeishuConfig
	logger *logrus.Logger
}

// NewFeishuNotifier 创建飞书通知器
func NewFeishuNotifier(config *types.FeishuConfig, logger *logrus.Logger) *FeishuNotifier {
	return &FeishuNotifier{
		config: config,
		logger: logger,
	}
}

// IsEnabled 检查是否启用
func (f *FeishuNotifier) IsEnabled() bool {
	return f.config.Enabled
}

// Send 发送飞书消息
func (f *FeishuNotifier) Send(alert *types.Alert) error {
	if !f.IsEnabled() {
		return nil
	}

	// 构建消息
	message := f.buildFeishuMessage(alert)

	// 发送请求
	webhookURL := f.config.WebhookURL
	if f.config.Secret != "" && f.config.Secret != "YOUR_SECRET" {
		webhookURL = f.addSign(webhookURL, f.config.Secret)
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送飞书消息失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容用于调试
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		f.logger.Errorf("飞书消息发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return fmt.Errorf("飞书消息发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	f.logger.Debugf("飞书消息发送成功，响应: %s", string(body))

	f.logger.Infof("飞书告警已发送: %s", alert.RuleName)
	return nil
}

// buildFeishuMessage 构建飞书消息
func (f *FeishuNotifier) buildFeishuMessage(alert *types.Alert) map[string]interface{} {
	// 构建@文本 - 只有严重告警才@用户
	atText := ""
	if f.shouldAtUser(alert.Level) {
		if f.config.AtAll {
			atText = "<at id=\"all\"></at>"
		} else if len(f.config.AtMobiles) > 0 {
			// 注意：这里需要真实的用户Open ID，手机号码无法直接@
			for _, mobile := range f.config.AtMobiles {
				atText += fmt.Sprintf("<at id=\"%s\"></at>", mobile)
			}
		}
	}

	// 构建卡片消息
	message := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": fmt.Sprintf("%s KubeSphere-OpenSearch 告警通知", f.getLevelEmoji(alert.Level)),
				},
				"template": f.getTemplateByLevel(alert.Level),
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("🏷️ **规则名称:** %s", alert.RuleName),
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("%s **告警级别:** %s", f.getLevelEmoji(alert.Level), alert.Level),
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("🕒 **触发时间:** %s", alert.Timestamp.Format("2006-01-02 15:04:05")),
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("📈 **匹配数量:** %d", alert.Count),
					},
				},
				{
					"tag": "hr",
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": f.formatMessageContent(alert.Message),
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": atText,
					},
				},
			},
		},
	}

	return message
}

// formatMessageContent 格式化消息内容，清理代码块和多余空白行
func (f *FeishuNotifier) formatMessageContent(message string) string {
	// 飞书 lark_md 格式，去掉代码块标记并清理空白行
	formatted := message

	// 移除代码块标记 ```
	formatted = strings.ReplaceAll(formatted, "```", "")

	// 清理多余的空行，将连续的换行符替换为最多两个换行符
	for strings.Contains(formatted, "\n\n\n") {
		formatted = strings.ReplaceAll(formatted, "\n\n\n", "\n\n")
	}

	// 清理行首行尾的空格
	lines := strings.Split(formatted, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" || (len(cleanLines) > 0 && cleanLines[len(cleanLines)-1] != "") {
			cleanLines = append(cleanLines, trimmed)
		}
	}
	formatted = strings.Join(cleanLines, "\n")

	// 确保开头和结尾没有多余的空行
	formatted = strings.TrimSpace(formatted)

	return formatted
}

// getTemplateByLevel 根据级别返回卡片主题色
func (f *FeishuNotifier) getTemplateByLevel(level string) string {
	switch level {
	case "Critical":
		return "red"
	case "High":
		return "orange"
	case "Medium":
		return "yellow"
	case "Low":
		return "green"
	case "Info":
		return "blue"
	default:
		return "red"
	}
}

// getLevelEmoji 不同级别对应的图标
func (f *FeishuNotifier) getLevelEmoji(level string) string {
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

// extractK8sInfo 提取K8s相关字段
func (f *FeishuNotifier) extractK8sInfo(alert *types.Alert) (podName, namespace, containerName, containerImage string) {
	if alert == nil || alert.Data == nil {
		return "", "", "", ""
	}
	sample, ok := alert.Data["sample_hit"].(map[string]interface{})
	if !ok {
		return "", "", "", ""
	}
	kube, ok := sample["kubernetes"].(map[string]interface{})
	if !ok {
		return "", "", "", ""
	}
	if v, ok := kube["pod_name"].(string); ok {
		podName = v
	}
	if v, ok := kube["namespace_name"].(string); ok {
		namespace = v
	}
	if v, ok := kube["container_name"].(string); ok {
		containerName = v
	}
	if v, ok := kube["container_image"].(string); ok {
		containerImage = v
	}
	return
}

// shouldAtUser 判断是否应该@用户
func (f *FeishuNotifier) shouldAtUser(level string) bool {
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
func (f *FeishuNotifier) addSign(webhookURL, secret string) string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	stringToSign := timestamp + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 飞书签名格式
	signStr := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", timestamp, sign)))

	// 这里需要根据实际的飞书 webhook URL 格式来添加签名
	// 飞书通常使用 timestamp 和 sign 作为查询参数
	return fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, signStr)
}
