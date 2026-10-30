package notificationchannel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Security string
}

type SMTPMailer struct {
	Config SMTPConfig
}

func NewSMTPMailer(config SMTPConfig) *SMTPMailer {
	return &SMTPMailer{Config: config}
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	config := m.Config
	if strings.TrimSpace(config.Host) == "" || strings.TrimSpace(config.From) == "" {
		return ErrEmailNotConfigured
	}
	port := config.Port
	if port <= 0 {
		port = 587
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || from.Address == "" {
		return fmt.Errorf("send email: invalid sender")
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address == "" {
		return fmt.Errorf("send email: invalid recipient")
	}
	address := net.JoinHostPort(config.Host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	security := strings.ToLower(strings.TrimSpace(config.Security))
	if security == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: config.Host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("send email: connect failed")
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("send email: smtp handshake failed")
	}
	defer client.Close()
	if security == "" || security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: config.Host}); err != nil {
				return fmt.Errorf("send email: starttls failed")
			}
		}
	}
	if config.Username != "" {
		auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("send email: auth failed")
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("send email: sender rejected")
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("send email: recipient rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("send email: data failed")
	}
	message := "From: " + from.Address + "\r\n" +
		"To: " + recipient.Address + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body + "\r\n"
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("send email: write failed")
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("send email: commit failed")
	}
	_ = client.Quit()
	return nil
}
