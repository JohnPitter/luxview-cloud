package model

import (
	"time"

	"github.com/google/uuid"
)

type Pageview struct {
	ID         int64      `json:"id"`
	AppID      *uuid.UUID `json:"app_id"`
	Timestamp  time.Time  `json:"timestamp"`
	Path       string     `json:"path"`
	Method     string     `json:"method"`
	StatusCode int        `json:"status_code"`
	IPHash     string     `json:"ip_hash"`
	Country    string     `json:"country,omitempty"`
	City       string     `json:"city,omitempty"`
	Region     string     `json:"region,omitempty"`
	Browser    string     `json:"browser,omitempty"`
	BrowserVer string     `json:"browser_ver,omitempty"`
	OS         string     `json:"os,omitempty"`
	DeviceType string     `json:"device_type,omitempty"`
	Referer    string     `json:"referer,omitempty"`
	ResponseMs int        `json:"response_ms"`
}

type PageviewAggregation struct {
	ID            int64      `json:"id"`
	AppID         *uuid.UUID `json:"app_id"`
	Bucket        time.Time  `json:"bucket"`
	Granularity   string     `json:"granularity"`
	Path          string     `json:"path,omitempty"`
	Views         int        `json:"views"`
	Visitors      int        `json:"visitors"`
	Bounces       int        `json:"bounces"`
	AvgDurationMs int        `json:"avg_duration_ms"`
	Country       string     `json:"country,omitempty"`
	Browser       string     `json:"browser,omitempty"`
	OS            string     `json:"os,omitempty"`
	DeviceType    string     `json:"device_type,omitempty"`
	RefererDomain string     `json:"referer_domain,omitempty"`
}
