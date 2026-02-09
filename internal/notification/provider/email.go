package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
)

// EmailProvider Email 通知提供者
type EmailProvider struct{}

// EmailConfig Email 配置
type EmailConfig struct {
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"from_address"`
	UseTLS      bool   `json:"use_tls"`
}

// NewEmailProvider 创建 Email 提供者
func NewEmailProvider() *EmailProvider {
	return &EmailProvider{}
}

// Type 返回提供者类型
func (p *EmailProvider) Type() string {
	return "email"
}

// Send 发送通知
func (p *EmailProvider) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := p.parseConfig(req.Config)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}

	if len(req.Recipients) == 0 {
		return &SendResponse{Success: false, ErrorMessage: "收件人列表为空"}, fmt.Errorf("收件人列表为空")
	}

	// 构建邮件内容
	subject := req.Subject
	if subject == "" {
		subject = "Auto-Healing 通知"
	}

	// 设置 Content-Type
	contentType := "text/plain; charset=UTF-8"
	if req.Format == "html" {
		contentType = "text/html; charset=UTF-8"
	}

	// 构建邮件头
	headers := make(map[string]string)
	headers["From"] = config.FromAddress
	headers["To"] = strings.Join(req.Recipients, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = contentType

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(req.Body)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)

	var sendErr error
	if config.UseTLS {
		sendErr = p.sendWithTLS(addr, auth, config, req.Recipients, []byte(message.String()))
	} else {
		sendErr = smtp.SendMail(addr, auth, config.FromAddress, req.Recipients, []byte(message.String()))
	}

	if sendErr != nil {
		return &SendResponse{Success: false, ErrorMessage: sendErr.Error()}, sendErr
	}

	return &SendResponse{Success: true}, nil
}

// sendWithTLS 使用 TLS 发送邮件
func (p *EmailProvider) sendWithTLS(addr string, auth smtp.Auth, config *EmailConfig, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         config.SMTPHost,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
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

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// Test 测试连接
func (p *EmailProvider) Test(ctx context.Context, configMap map[string]interface{}) error {
	config, err := p.parseConfig(configMap)
	if err != nil {
		return err
	}

	// 测试 SMTP 连接
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)

	var conn interface{}
	var connErr error

	if config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         config.SMTPHost,
		}
		conn, connErr = tls.Dial("tcp", addr, tlsConfig)
	} else {
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("SMTP 连接失败: %w", err)
		}
		defer client.Close()

		// 尝试认证
		auth := smtp.PlainAuth("", config.Username, config.Password, config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
		return nil
	}

	if connErr != nil {
		return fmt.Errorf("TLS 连接失败: %w", connErr)
	}

	if tlsConn, ok := conn.(interface{ Close() error }); ok {
		defer tlsConn.Close()
	}

	return nil
}

// parseConfig 解析配置
func (p *EmailProvider) parseConfig(configMap map[string]interface{}) (*EmailConfig, error) {
	jsonData, err := json.Marshal(configMap)
	if err != nil {
		return nil, err
	}

	var config EmailConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}

	if config.SMTPHost == "" {
		return nil, fmt.Errorf("smtp_host 不能为空")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if config.FromAddress == "" {
		config.FromAddress = config.Username
	}

	return &config, nil
}
