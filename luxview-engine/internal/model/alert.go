package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AlertChannel string

const (
	AlertEmail   AlertChannel = "email"
	AlertWebhook AlertChannel = "webhook"
	AlertDiscord AlertChannel = "discord"
)

type Alert struct {
	ID            uuid.UUID       `json:"id"`
	AppID         uuid.UUID       `json:"app_id"`
	Metric        string          `json:"metric"`    // cpu_percent, memory_bytes, etc.
	Condition     string          `json:"condition"` // gt, lt, gte, lte, eq
	Threshold     float64         `json:"threshold"`
	Channel       AlertChannel    `json:"channel"`
	ChannelConfig json.RawMessage `json:"channel_config"`
	Enabled       bool            `json:"enabled"`
	LastTriggered *time.Time      `json:"last_triggered,omitempty"`
}

type CreateAlertRequest struct {
	Metric        string          `json:"metric"`
	Condition     string          `json:"condition"`
	Threshold     float64         `json:"threshold"`
	Channel       AlertChannel    `json:"channel"`
	ChannelConfig json.RawMessage `json:"channel_config"`
}

type UpdateAlertRequest struct {
	Metric        *string          `json:"metric,omitempty"`
	Condition     *string          `json:"condition,omitempty"`
	Threshold     *float64         `json:"threshold,omitempty"`
	Channel       *AlertChannel    `json:"channel,omitempty"`
	ChannelConfig json.RawMessage  `json:"channel_config,omitempty"`
	Enabled       *bool            `json:"enabled,omitempty"`
}
