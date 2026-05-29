package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"time"

	"github.com/luxview/engine/internal/config"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// Alerter evaluates alert rules against the latest metrics and triggers notifications.
type Alerter struct {
	alertRepo  *repository.AlertRepo
	metricRepo *repository.MetricRepo
	appRepo    *repository.AppRepo
	userRepo   *repository.UserRepo
	smtpCfg    smtpConfig
	client     *http.Client
}

type smtpConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

func NewAlerter(alertRepo *repository.AlertRepo, metricRepo *repository.MetricRepo, appRepo *repository.AppRepo, userRepo *repository.UserRepo, cfg *config.Config) *Alerter {
	return &Alerter{
		alertRepo:  alertRepo,
		metricRepo: metricRepo,
		appRepo:    appRepo,
		userRepo:   userRepo,
		smtpCfg: smtpConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		},
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// EvaluateAll checks all enabled alerts against latest metrics.
func (a *Alerter) EvaluateAll(ctx context.Context) {
	log := logger.With("alerter")

	alerts, err := a.alertRepo.ListAllEnabled(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to list alerts")
		return
	}

	log.Info().Int("count", len(alerts)).Msg("evaluating alerts")

	for _, alert := range alerts {
		triggered, err := a.evaluate(ctx, &alert)
		if err != nil {
			log.Warn().Err(err).Str("alert_id", alert.ID.String()).Msg("failed to evaluate alert")
			continue
		}
		if triggered {
			log.Info().Str("alert_id", alert.ID.String()).Str("metric", alert.Metric).Msg("alert condition met, triggering")
			if err := a.trigger(ctx, &alert); err != nil {
				log.Error().Err(err).Str("alert_id", alert.ID.String()).Msg("failed to trigger alert")
			}
		}
	}
}

func (a *Alerter) evaluate(ctx context.Context, alert *model.Alert) (bool, error) {
	metric, err := a.metricRepo.GetLatest(ctx, alert.AppID)
	if err != nil {
		return false, err
	}

	var value float64
	switch alert.Metric {
	case "cpu_percent":
		value = metric.CPUPercent
	case "memory_bytes":
		value = float64(metric.MemoryBytes)
	case "network_rx":
		value = float64(metric.NetworkRx)
	case "network_tx":
		value = float64(metric.NetworkTx)
	default:
		return false, fmt.Errorf("unknown metric: %s", alert.Metric)
	}

	switch alert.Condition {
	case "gt":
		return value > alert.Threshold, nil
	case "gte":
		return value >= alert.Threshold, nil
	case "lt":
		return value < alert.Threshold, nil
	case "lte":
		return value <= alert.Threshold, nil
	case "eq":
		return value == alert.Threshold, nil
	default:
		return false, fmt.Errorf("unknown condition: %s", alert.Condition)
	}
}

var metricLabels = map[string]string{
	"cpu_percent":  "CPU (%)",
	"memory_bytes": "Memória (bytes)",
	"network_rx":   "Rede Entrada (bytes/s)",
	"network_tx":   "Rede Saída (bytes/s)",
}

var conditionLabels = map[string]string{
	"gt":  ">",
	"gte": ">=",
	"lt":  "<",
	"lte": "<=",
	"eq":  "=",
}

func (a *Alerter) trigger(ctx context.Context, alert *model.Alert) error {
	log := logger.With("alerter")

	// Cooldown: don't re-trigger within 5 minutes
	if alert.LastTriggered != nil && time.Since(*alert.LastTriggered) < 5*time.Minute {
		return nil
	}

	app, err := a.appRepo.FindByID(ctx, alert.AppID)
	if err != nil || app == nil {
		return fmt.Errorf("app not found for alert")
	}

	metricLabel := metricLabels[alert.Metric]
	if metricLabel == "" {
		metricLabel = alert.Metric
	}
	condLabel := conditionLabels[alert.Condition]
	if condLabel == "" {
		condLabel = alert.Condition
	}

	// Get current value for context
	currentValue := 0.0
	if m, err := a.metricRepo.GetLatest(ctx, alert.AppID); err == nil {
		switch alert.Metric {
		case "cpu_percent":
			currentValue = m.CPUPercent
		case "memory_bytes":
			currentValue = float64(m.MemoryBytes)
		case "network_rx":
			currentValue = float64(m.NetworkRx)
		case "network_tx":
			currentValue = float64(m.NetworkTx)
		}
	}

	// Plain text for webhook/discord
	plainMessage := fmt.Sprintf("[LuxView] Alerta: %s %s %.1f na aplicação %s — Valor atual: %.1f",
		metricLabel, condLabel, alert.Threshold, app.Name, currentValue)

	// HTML email body
	emailBody := fmt.Sprintf(`<div style="font-family:system-ui,sans-serif;max-width:520px;margin:0 auto;padding:24px;background:#18181b;border-radius:12px;color:#e4e4e7">
  <div style="text-align:center;margin-bottom:20px">
    <span style="font-size:28px">⚠️</span>
    <h2 style="margin:8px 0 0;color:#fbbf24;font-size:18px">Alerta Disparado</h2>
  </div>
  <table style="width:100%%;border-collapse:collapse;font-size:14px">
    <tr><td style="padding:8px 0;color:#a1a1aa">Aplicação</td><td style="padding:8px 0;color:#f4f4f5;font-weight:600">%s</td></tr>
    <tr><td style="padding:8px 0;color:#a1a1aa">Métrica</td><td style="padding:8px 0;color:#f4f4f5">%s</td></tr>
    <tr><td style="padding:8px 0;color:#a1a1aa">Condição</td><td style="padding:8px 0;color:#f4f4f5">%s %.1f</td></tr>
    <tr><td style="padding:8px 0;color:#a1a1aa">Valor Atual</td><td style="padding:8px 0;color:#ef4444;font-weight:600">%.1f</td></tr>
    <tr><td style="padding:8px 0;color:#a1a1aa">Horário</td><td style="padding:8px 0;color:#f4f4f5">%s</td></tr>
  </table>
  <hr style="border:none;border-top:1px solid #27272a;margin:16px 0">
  <p style="font-size:12px;color:#71717a;text-align:center;margin:0">LuxView Cloud — Monitoramento</p>
</div>`, app.Name, metricLabel, condLabel, alert.Threshold, currentValue, time.Now().Format("02/01/2006 15:04:05"))

	switch alert.Channel {
	case model.AlertWebhook:
		if err := a.sendWebhook(alert.ChannelConfig, plainMessage); err != nil {
			return err
		}
	case model.AlertDiscord:
		if err := a.sendDiscord(alert.ChannelConfig, plainMessage); err != nil {
			return err
		}
	case model.AlertEmail:
		recipient, err := a.resolveEmailRecipient(ctx, alert)
		if err != nil {
			return fmt.Errorf("resolve email recipient: %w", err)
		}
		if err := a.sendEmail(recipient, app.Name, emailBody); err != nil {
			return err
		}
	}

	_ = a.alertRepo.UpdateLastTriggered(ctx, alert.ID)
	log.Info().Str("alert_id", alert.ID.String()).Str("app", app.Subdomain).Msg("alert triggered")
	return nil
}

func (a *Alerter) sendWebhook(channelConfig json.RawMessage, message string) error {
	var cfg struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(channelConfig, &cfg); err != nil || cfg.URL == "" {
		return fmt.Errorf("invalid webhook config")
	}

	payload, _ := json.Marshal(map[string]string{"text": message})
	resp, err := a.client.Post(cfg.URL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (a *Alerter) sendDiscord(channelConfig json.RawMessage, message string) error {
	var cfg struct {
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.Unmarshal(channelConfig, &cfg); err != nil || cfg.WebhookURL == "" {
		return fmt.Errorf("invalid discord config")
	}

	payload, _ := json.Marshal(map[string]string{"content": message})
	resp, err := a.client.Post(cfg.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (a *Alerter) resolveEmailRecipient(ctx context.Context, alert *model.Alert) (string, error) {
	var cfg struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(alert.ChannelConfig, &cfg); err == nil && cfg.Target != "" {
		return cfg.Target, nil
	}

	app, err := a.appRepo.FindByID(ctx, alert.AppID)
	if err != nil || app == nil {
		return "", fmt.Errorf("app not found")
	}
	user, err := a.userRepo.FindByID(ctx, app.UserID)
	if err != nil || user == nil {
		return "", fmt.Errorf("user not found")
	}
	return user.Email, nil
}

func (a *Alerter) sendEmail(to, appName, message string) error {
	log := logger.With("alerter")

	if a.smtpCfg.Host == "" {
		return fmt.Errorf("SMTP not configured (SMTP_HOST is empty)")
	}

	subject := fmt.Sprintf("LuxView Alert — %s", appName)
	msgID := fmt.Sprintf("<%d.alert@luxview.cloud>", time.Now().UnixNano())
	body := fmt.Sprintf("From: LuxView Alerts <%s>\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=utf-8\r\n\r\n%s",
		a.smtpCfg.From, to, subject, time.Now().Format(time.RFC1123Z), msgID, message)

	addr := fmt.Sprintf("%s:%d", a.smtpCfg.Host, a.smtpCfg.Port)
	log.Info().Str("to", to).Str("host", addr).Msg("sending alert email")

	// Connect via plain TCP (internal docker network, no TLS needed)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}

	client, err := smtp.NewClient(conn, a.smtpCfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// Try STARTTLS if available, skip if not (internal network)
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: a.smtpCfg.Host, InsecureSkipVerify: true}
		if err := client.StartTLS(tlsCfg); err != nil {
			log.Warn().Err(err).Msg("STARTTLS failed, continuing without TLS")
		}
	}

	// Authenticate only if STARTTLS succeeded (PlainAuth requires encryption)
	if a.smtpCfg.User != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", a.smtpCfg.User, a.smtpCfg.Password, a.smtpCfg.Host)
			if err := client.Auth(auth); err != nil {
				log.Warn().Err(err).Msg("smtp auth failed, continuing without auth")
			}
		}
	}

	if err := client.Mail(a.smtpCfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	return client.Quit()
}
