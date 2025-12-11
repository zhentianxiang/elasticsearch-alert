package notification

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type EmailNotifier struct {
	Host          string
	Port          int
	Username      string
	Password      string
	From          string
	To            []string
	UseTLS        bool
	TLSSkipVerify bool
	SubjectPrefix string
	Timeout       time.Duration
}

func (e *EmailNotifier) Name() string { return "email" }

func (e *EmailNotifier) Send(ctx context.Context, title, text string) error {
	subject := title
	if e.SubjectPrefix != "" {
		subject = e.SubjectPrefix + " " + title
	}
	msg := buildEmailMessage(e.From, e.To, subject, text)
	addr := fmt.Sprintf("%s:%d", e.Host, e.Port)
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)

	dialer := net.Dialer{Timeout: e.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if e.UseTLS {
		tlsCfg := &tls.Config{
			ServerName:         e.Host,
			InsecureSkipVerify: e.TLSSkipVerify,
		}
		tlsConn := tls.Client(conn, tlsCfg)
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		c, err := smtp.NewClient(tlsConn, e.Host)
		if err != nil {
			return err
		}
		defer c.Quit()
		if e.Username != "" {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
		if err := sendMail(c, e.From, e.To, []byte(msg)); err != nil {
			return err
		}
		return nil
	}

	// STARTTLS path
	c, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		return err
	}
	defer c.Quit()

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{
			ServerName:         e.Host,
			InsecureSkipVerify: e.TLSSkipVerify,
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if e.Username != "" {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := sendMail(c, e.From, e.To, []byte(msg)); err != nil {
		return err
	}
	return nil
}

func buildEmailMessage(from string, to []string, subject, body string) string {
	// 参考 opensearch-alert-main，将邮件内容美化为简单的 HTML 卡片，并支持少量 Markdown（**加粗**、换行）
	formattedBody := markdownToHTML(body)
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>%s</title>
  <style>
    body { font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif; margin: 20px; color: #333; }
    .card { border-radius: 10px; border: 1px solid #f5c6cb; background-color: #fdecea; padding: 16px 20px; margin-bottom: 20px; }
    .card h2 { margin: 0 0 8px 0; }
    .content { background: #f8f9fa; border-radius: 6px; padding: 12px 16px; white-space: pre-wrap; font-family: Menlo,Consolas,monospace; }
  </style>
</head>
<body>
  <div class="card">
    <h2>🚨 Elasticsearch 日志告警</h2>
    <div>%s</div>
  </div>
  <div class="content">%s</div>
</body>
</html>
`, html.EscapeString(subject), html.EscapeString(subject), formattedBody)

	headers := map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}
	var sb strings.Builder
	for k, v := range headers {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")
	sb.WriteString(htmlBody)
	return sb.String()
}

// markdownToHTML 将非常简单的 Markdown（**加粗**、\n 换行）转换为 HTML 片段
func markdownToHTML(s string) string {
	var b strings.Builder
	inBold := false
	for i := 0; i < len(s); {
		// 处理粗体 **
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			if inBold {
				b.WriteString("</strong>")
			} else {
				b.WriteString("<strong>")
			}
			inBold = !inBold
			i += 2
			continue
		}
		ch := s[i]
		switch ch {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\n':
			b.WriteString("<br>")
		default:
			b.WriteByte(ch)
		}
		i++
	}
	if inBold {
		b.WriteString("</strong>")
	}
	return b.String()
}

func sendMail(c *smtp.Client, from string, to []string, msg []byte) error {
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}


