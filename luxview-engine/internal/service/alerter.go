package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"github.com/luxview/engine/pkg/logger"
)

// Alerter evaluates alert rules against the latest metrics and triggers notifications.
type Alerter struct {
	alertRepo  *repository.AlertRepo
	metricRepo *repository.MetricRepo
	appRepo    *repository.AppRepo
	client     *http.Client
}

func NewAlerter(alertRepo *repository.AlertRepo, metricRepo *repository.MetricRepo, appRepo *repository.AppRepo) *Alerter {
	return &Alerter{
		alertRepo:  alertRepo,
		metricRepo: metricRepo,
		appRepo:    appRepo,
		client:     &http.Client{Timeout: 10 * time.Second},
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

	for _, alert := range alerts {
		triggered, err := a.evaluate(ctx, &alert)
		if err != nil {
			log.Debug().Err(err).Str("alert_id", alert.ID.String()).Msg("failed to evaluate alert")
			continue
		}
		if triggered {
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

	message := fmt.Sprintf("Alert: %s %s %.2f for app %s (%s)",
		alert.Metric, alert.Condition, alert.Threshold, app.Name, app.Subdomain)

	switch alert.Channel {
	case model.AlertWebhook:
		if err := a.sendWebhook(alert.ChannelConfig, message); err != nil {
			return err
		}
	case model.AlertDiscord:
		if err := a.sendDiscord(alert.ChannelConfig, message); err != nil {
			return err
		}
	case model.AlertEmail:
		log.Info().Str("message", message).Msg("email alert (not implemented)")
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
