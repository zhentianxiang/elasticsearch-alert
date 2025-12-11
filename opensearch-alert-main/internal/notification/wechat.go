package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"opensearch-alert/pkg/types"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// WeChatNotifier 企业微信通知器
type WeChatNotifier struct {
	config *types.WeChatConfig
	logger *logrus.Logger
}

// NewWeChatNotifier 创建企业微信通知器
func NewWeChatNotifier(config *types.WeChatConfig, logger *logrus.Logger) *WeChatNotifier {
	return &WeChatNotifier{
		config: config,
		logger: logger,
	}
}

// IsEnabled 检查是否启用
func (w *WeChatNotifier) IsEnabled() bool {
	return w.config.Enabled
}

// Send 发送企业微信消息
func (w *WeChatNotifier) Send(alert *types.Alert) error {
	if !w.IsEnabled() {
		return nil
	}

	// 构建消息
	message := w.buildWeChatMessage(alert)

	// 发送请求
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := http.Post(w.config.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送企业微信消息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("企业微信消息发送失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应内容用于调试
	body, _ := io.ReadAll(resp.Body)
	w.logger.Debugf("企业微信消息发送成功，响应: %s", string(body))

	w.logger.Infof("企业微信告警已发送: %s", alert.RuleName)
	return nil
}

// buildWeChatMessage 构建企业微信消息
func (w *WeChatNotifier) buildWeChatMessage(alert *types.Alert) map[string]interface{} {
	// 构建文本内容，使用表情+标签格式，并包含简要详情
	content := fmt.Sprintf("%s KubeSphere-OpenSearch 告警通知\n\n"+
		"🏷️ 规则: %s\n"+
		"%s 级别: %s\n"+
		"🕒 时间: %s\n"+
		"📈 匹配: %d\n\n"+
		"📝 详情:\n%s",
		w.getLevelEmoji(alert.Level), alert.RuleName,
		w.getLevelEmoji(alert.Level), alert.Level,
		alert.Timestamp.Format("2006-01-02 15:04:05"),
		alert.Count, w.formatMessageContent(alert.Message))

	// 构建消息体
	message := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": content,
		},
	}

	// 添加@用户信息（企业微信text格式支持@功能）
	mentionedList := []string{}
	mentionedMobileList := []string{}

	// 只有严重告警才@用户
	if w.shouldAtUser(alert.Level) {
		// 如果配置了@所有人，则@所有人
		if w.config.AtAll {
			mentionedList = []string{"@all"}
			// 注意：@所有人时只设置mentioned_list，不设置mentioned_mobile_list
		} else {
			// 使用配置的用户ID和手机号码
			if len(w.config.MentionedList) > 0 {
				mentionedList = w.config.MentionedList
			}
			if len(w.config.MentionedMobileList) > 0 {
				mentionedMobileList = w.config.MentionedMobileList
			}
		}
	}

	// 只设置非空的字段
	if len(mentionedList) > 0 {
		message["text"].(map[string]interface{})["mentioned_list"] = mentionedList
	}
	if len(mentionedMobileList) > 0 {
		message["text"].(map[string]interface{})["mentioned_mobile_list"] = mentionedMobileList
	}

	return message
}

// formatMessageContent 格式化消息内容，将Markdown格式转换为纯文本
func (w *WeChatNotifier) formatMessageContent(message string) string {
	// 将Markdown格式转换为纯文本格式
	formatted := message

	// 移除粗体标记 **text** -> text
	formatted = strings.ReplaceAll(formatted, "**", "")

	// 移除代码块标记 ``` -> 空行
	formatted = strings.ReplaceAll(formatted, "```", "")

	// 移除分隔线标记 '---' 以及日志中仅由横线组成的分割线
	formatted = strings.ReplaceAll(formatted, "---", "")
	hyphenDivider := regexp.MustCompile(`(?m)^\s*-{6,}\s*$`)
	formatted = hyphenDivider.ReplaceAllString(formatted, "")

	// 清理多余的空行（将3个及以上连续换行压缩为2个）
	multiEmptyLines := regexp.MustCompile(`\n{3,}`)
	formatted = multiEmptyLines.ReplaceAllString(formatted, "\n\n")

	// 确保开头和结尾没有多余的空行
	formatted = strings.TrimSpace(formatted)

	return formatted
}

// getLevelEmoji 不同级别对应的图标
func (w *WeChatNotifier) getLevelEmoji(level string) string {
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

// extractK8sInfo 从 alert.Data.sample_hit 中提取 K8s 相关信息
// 原格式化函数保留以便将来启用消息详情时复用

// shouldAtUser 判断是否应该@用户
func (w *WeChatNotifier) shouldAtUser(level string) bool {
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
