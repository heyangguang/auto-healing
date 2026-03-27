package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strconv"
	"strings"

	settingsrepo "github.com/company/auto-healing/internal/platform/repository/settings"
)

// ==================== 平台级邮件服务 ====================
// 独立于租户通知渠道，从 PlatformSettings 读取 SMTP 配置，
// 用于发送平台级事务性邮件（如邀请邮件）。

// PlatformEmailService 平台级邮件服务
type PlatformEmailService struct {
	settingsRepo *settingsrepo.PlatformSettingsRepository
}

type PlatformEmailServiceDeps struct {
	SettingsRepo *settingsrepo.PlatformSettingsRepository
}

// NewPlatformEmailService 创建平台邮件服务
func NewPlatformEmailService() *PlatformEmailService {
	return NewPlatformEmailServiceWithDeps(PlatformEmailServiceDeps{
		SettingsRepo: settingsrepo.NewPlatformSettingsRepository(),
	})
}

func NewPlatformEmailServiceWithDeps(deps PlatformEmailServiceDeps) *PlatformEmailService {
	return &PlatformEmailService{
		settingsRepo: deps.SettingsRepo,
	}
}

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	UseTLS      bool
}

// IsConfigured 检查平台邮箱是否已配置
func (s *PlatformEmailService) IsConfigured(ctx context.Context) bool {
	host := s.settingsRepo.GetStringValue(ctx, "email.smtp_host", "")
	return host != ""
}

// GetConfig 获取当前 SMTP 配置
func (s *PlatformEmailService) GetConfig(ctx context.Context) (*SMTPConfig, error) {
	host := s.settingsRepo.GetStringValue(ctx, "email.smtp_host", "")
	if host == "" {
		return nil, fmt.Errorf("平台邮箱服务未配置，请在平台设置中配置 SMTP 参数")
	}

	return &SMTPConfig{
		Host:        host,
		Port:        s.settingsRepo.GetIntValue(ctx, "email.smtp_port", 587),
		Username:    s.settingsRepo.GetStringValue(ctx, "email.username", ""),
		Password:    s.settingsRepo.GetStringValue(ctx, "email.password", ""),
		FromAddress: s.settingsRepo.GetStringValue(ctx, "email.from_address", ""),
		UseTLS:      s.settingsRepo.GetBoolValue(ctx, "email.use_tls", true),
	}, nil
}

// SendInvitationEmail 发送邀请邮件
func (s *PlatformEmailService) SendInvitationEmail(ctx context.Context, to, tenantName, roleName, inviteURL string) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("邀请加入「%s」— Auto-Healing 平台", tenantName)

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
  <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; border-radius: 8px 8px 0 0; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 24px;">Auto-Healing 平台</h1>
    <p style="color: rgba(255,255,255,0.9); margin: 8px 0 0;">智能自愈运维平台</p>
  </div>
  <div style="background: #fff; padding: 30px; border: 1px solid #e8e8e8; border-top: none; border-radius: 0 0 8px 8px;">
    <h2 style="color: #262626; margin-top: 0;">您被邀请加入租户</h2>
    <p style="color: #595959; line-height: 1.6;">
      您好，您已被邀请加入 <strong>「%s」</strong> 租户，角色为 <strong>%s</strong>。
    </p>
    <p style="color: #595959; line-height: 1.6;">请点击下方按钮完成注册：</p>
    <div style="text-align: center; margin: 30px 0;">
      <a href="%s" style="display: inline-block; padding: 12px 32px; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; text-decoration: none; border-radius: 6px; font-size: 16px; font-weight: 500;">
        接受邀请并注册
      </a>
    </div>
    <p style="color: #8c8c8c; font-size: 13px;">
      如果按钮无法点击，请复制以下链接到浏览器：<br/>
      <a href="%s" style="color: #667eea; word-break: break-all;">%s</a>
    </p>
    <hr style="border: none; border-top: 1px solid #f0f0f0; margin: 20px 0;"/>
    <p style="color: #bfbfbf; font-size: 12px; text-align: center;">
      此邮件由 Auto-Healing 平台自动发送，请勿直接回复。
    </p>
  </div>
</body>
</html>`, tenantName, roleName, inviteURL, inviteURL, inviteURL)

	return s.sendEmail(config, to, subject, body, "html")
}

// sendEmail 底层邮件发送
func (s *PlatformEmailService) sendEmail(config *SMTPConfig, to, subject, body, format string) error {
	if config.FromAddress == "" {
		config.FromAddress = config.Username
	}

	// 构建邮件头
	contentType := "text/plain; charset=UTF-8"
	if format == "html" {
		contentType = "text/html; charset=UTF-8"
	}

	headers := map[string]string{
		"From":         config.FromAddress,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": contentType,
	}

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return s.sendWithTLS(addr, auth, config, []string{to}, []byte(message.String()))
	}
	return smtp.SendMail(addr, auth, config.FromAddress, []string{to}, []byte(message.String()))
}

// sendWithTLS TLS 发送
func (s *PlatformEmailService) sendWithTLS(addr string, auth smtp.Auth, config *SMTPConfig, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("SMTP 客户端创建失败: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}
	if err = client.Mail(config.FromAddress); err != nil {
		return err
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// TestConnection 测试 SMTP 连接
func (s *PlatformEmailService) TestConnection(ctx context.Context) error {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         config.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS 连接失败: %w", err)
		}
		conn.Close()
		return nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP 连接失败: %w", err)
	}
	defer client.Close()

	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}
	return nil
}

// Helper: 从字符串安全转 int
func safeAtoi(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func init() {
	// 抑制未使用的导入
	_ = log.Ldate
	_ = safeAtoi
}
