package notification

import (
	"fmt"
	"opensearch-alert/pkg/types"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Notifier 通知器
type Notifier struct {
	email    *EmailNotifier
	dingtalk *DingTalkNotifier
	wechat   *WeChatNotifier
	feishu   *FeishuNotifier
	logger   *logrus.Logger
}

// NewNotifier 创建新的通知器
func NewNotifier(config *types.Config, logger *logrus.Logger) *Notifier {
	return &Notifier{
		email:    NewEmailNotifier(&config.Notifications.Email, logger),
		dingtalk: NewDingTalkNotifier(&config.Notifications.DingTalk, logger),
		wechat:   NewWeChatNotifier(&config.Notifications.WeChat, logger),
		feishu:   NewFeishuNotifier(&config.Notifications.Feishu, logger),
		logger:   logger,
	}
}

// SendAlert 发送告警
func (n *Notifier) SendAlert(alert *types.Alert) error {
	n.logger.Debugf("开始发送告警: %s (级别: %s)", alert.RuleName, alert.Level)

	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex

	// 并发发送到所有启用的通知渠道
	if n.email.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.email.Send(alert); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}

	if n.dingtalk.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.dingtalk.Send(alert); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}

	if n.wechat.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.wechat.Send(alert); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}

	if n.feishu.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.feishu.Send(alert); err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// 如果有错误，记录但不中断流程
	if len(errors) > 0 {
		n.logger.Errorf("部分通知发送失败: %v", errors)
	}

	return nil
}

// TestNotifications 测试所有启用的通知渠道
func (n *Notifier) TestNotifications() error {
	// 创建测试告警
	testAlert := &types.Alert{
		ID:        "test-alert-" + fmt.Sprintf("%d", time.Now().Unix()),
		RuleName:  "连接测试",
		Level:     "Info",
		Message:   "这是一条测试消息，用于验证通知渠道是否正常工作。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test":    true,
			"message": "OpenSearch 告警工具连接测试成功",
		},
		Count:   1,
		Matches: 1,
	}

	n.logger.Info("开始测试通知渠道...")

	var wg sync.WaitGroup
	var errors []error
	var mu sync.Mutex

	// 测试邮件通知
	if n.email.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.logger.Info("测试邮件通知...")
			if err := n.email.Send(testAlert); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("邮件通知测试失败: %w", err))
				mu.Unlock()
			} else {
				n.logger.Info("✅ 邮件通知测试成功")
			}
		}()
	}

	// 测试钉钉通知
	if n.dingtalk.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.logger.Info("测试钉钉通知...")
			if err := n.dingtalk.Send(testAlert); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("钉钉通知测试失败: %w", err))
				mu.Unlock()
			} else {
				n.logger.Info("✅ 钉钉通知测试成功")
			}
		}()
	}

	// 测试企业微信通知
	if n.wechat.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.logger.Info("测试企业微信通知...")
			if err := n.wechat.Send(testAlert); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("企业微信通知测试失败: %w", err))
				mu.Unlock()
			} else {
				n.logger.Info("✅ 企业微信通知测试成功")
			}
		}()
	}

	// 测试飞书通知
	if n.feishu.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.logger.Info("测试飞书通知...")
			if err := n.feishu.Send(testAlert); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("飞书通知测试失败: %w", err))
				mu.Unlock()
			} else {
				n.logger.Info("✅ 飞书通知测试成功")
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("部分通知渠道测试失败: %v", errors)
	}

	n.logger.Info("🎉 所有启用的通知渠道测试完成")
	return nil
}
