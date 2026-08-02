package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WebhookURL   string   `json:"webhook_url"`
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     int      `json:"smtp_port"`
	SMTPUsername string   `json:"smtp_username"`
	SMTPPassword string   `json:"smtp_password"`
	SMTPFrom     string   `json:"smtp_from"`
	SMTPTo       []string `json:"smtp_to"`
}
type Event struct {
	Title    string    `json:"title"`
	Message  string    `json:"message"`
	Severity string    `json:"severity"`
	Time     time.Time `json:"time"`
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}
func Send(cfg Config, e Event) error {
	if cfg.WebhookURL == "" && (cfg.SMTPHost == "" || len(cfg.SMTPTo) == 0) {
		return fmt.Errorf("no notification channel is configured")
	}
	errs := []string{}
	if cfg.WebhookURL != "" {
		if err := webhook(cfg.WebhookURL, e); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if cfg.SMTPHost != "" && len(cfg.SMTPTo) > 0 {
		if err := mail(cfg, e); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("notification failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
func webhook(url string, e Event) error {
	b, _ := json.Marshal(e)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}
func mail(cfg Config, e Event) error {
	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if cfg.SMTPUsername != "" {
		return fmt.Errorf("SMTP server does not offer STARTTLS")
	}
	if cfg.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
			return err
		}
	}
	from := cfg.SMTPFrom
	if from == "" {
		from = cfg.SMTPUsername
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, to := range cfg.SMTPTo {
		if err := client.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	subject := strings.ReplaceAll(e.Title, "\n", "")
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: [AnPanel] %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", from, strings.Join(cfg.SMTPTo, ","), subject, e.Message)
	if _, err = w.Write([]byte(body)); err != nil {
		return err
	}
	return w.Close()
}
