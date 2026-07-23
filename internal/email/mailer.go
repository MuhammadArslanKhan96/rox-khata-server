package email

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Config holds SMTP server settings.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Mailer handles sending emails.
type Mailer struct {
	config Config
}

// NewMailer initializes mailer configuration from environment variables.
func NewMailer() *Mailer {
	return &Mailer{
		config: Config{
			Host:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			Port:     getEnv("SMTP_PORT", "587"),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "Rox Khata <noreply@roxkhata.com>"),
		},
	}
}

// SendVerificationEmail sends a 6-digit OTP verification email to the newly registered shop owner.
func (m *Mailer) SendVerificationEmail(toEmail, businessName, otpCode string) error {
	if toEmail == "" {
		return fmt.Errorf("recipient email address is empty")
	}

	subject := fmt.Sprintf("Subject: Verify Your Rox Khata Account - %s\n", otpCode)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<style>
  body { font-family: 'Segoe UI', Arial, sans-serif; background-color: #f4f6f8; margin: 0; padding: 20px; }
  .card { max-width: 520px; margin: 0 auto; background: #ffffff; border-radius: 12px; padding: 32px; box-shadow: 0 4px 12px rgba(0,0,0,0.08); border: 1px solid #e2e8f0; }
  .logo { font-size: 26px; font-weight: 800; color: #1E3A8A; text-align: center; margin-bottom: 8px; }
  .subtitle { font-size: 14px; color: #64748b; text-align: center; margin-bottom: 24px; }
  .greeting { font-size: 16px; color: #1e293b; font-weight: 600; margin-bottom: 16px; }
  .code-box { background: #eff6ff; border: 2px dashed #1E3A8A; border-radius: 10px; padding: 20px; text-align: center; margin: 24px 0; }
  .otp { font-size: 34px; font-weight: 900; letter-spacing: 8px; color: #1E3A8A; }
  .footer { font-size: 12px; color: #94a3b8; text-align: center; margin-top: 32px; border-top: 1px solid #f1f5f9; padding-top: 16px; }
</style>
</head>
<body>
  <div class="card">
    <div class="logo">Rox Khata</div>
    <div class="subtitle">Smart Digital Ledger & Business Management</div>
    <div class="greeting">Welcome %s!</div>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">
      Thank you for registering your business with <strong>Rox Khata</strong>. Please use the verification code below to complete your sign-up process:
    </p>
    <div class="code-box">
      <div class="otp">%s</div>
    </div>
    <p style="color: #64748b; font-size: 13px;">
      This code is valid for 15 minutes. If you did not initiate this request, please ignore this email.
    </p>
    <div class="footer">
      &copy; %d Roxan Labs. All rights reserved.
    </div>
  </div>
</body>
</html>`, businessName, otpCode, time.Now().Year())

	msg := []byte(subject + mime + body)

	// If no SMTP credentials configured, log email for testing
	if m.config.Username == "" || m.config.Password == "" {
		log.Printf("[Email Notice] SMTP credentials not set in .env. Verification OTP [%s] for [%s] (%s):\n", otpCode, businessName, toEmail)
		return nil
	}

	return m.sendSMTP(toEmail, msg)
}

func (m *Mailer) sendSMTP(toEmail string, msg []byte) error {
	addr := net.JoinHostPort(m.config.Host, m.config.Port)
	auth := smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)

	// 587 uses STARTTLS
	if m.config.Port == "587" {
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to dial SMTP server: %w", err)
		}
		defer c.Close()

		if err = c.StartTLS(&tls.Config{ServerName: m.config.Host, InsecureSkipVerify: true}); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}

		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}

		if err = c.Mail(m.extractEmail(m.config.From)); err != nil {
			return fmt.Errorf("failed to set MAIL FROM: %w", err)
		}

		if err = c.Rcpt(toEmail); err != nil {
			return fmt.Errorf("failed to set RCPT TO: %w", err)
		}

		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("failed to open data writer: %w", err)
		}

		_, err = w.Write(msg)
		if err != nil {
			return fmt.Errorf("failed to write email body: %w", err)
		}

		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return c.Quit()
	}

	// Default fallback (465 SSL or standard)
	return smtp.SendMail(addr, auth, m.extractEmail(m.config.From), []string{toEmail}, msg)
}

func (m *Mailer) extractEmail(fromHeader string) string {
	if strings.Contains(fromHeader, "<") && strings.Contains(fromHeader, ">") {
		start := strings.Index(fromHeader, "<") + 1
		end := strings.Index(fromHeader, ">")
		return fromHeader[start:end]
	}
	return fromHeader
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
