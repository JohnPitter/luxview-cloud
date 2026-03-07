package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ServiceType string

const (
	ServicePostgres ServiceType = "postgres"
	ServiceRedis    ServiceType = "redis"
	ServiceMongoDB  ServiceType = "mongodb"
	ServiceRabbitMQ ServiceType = "rabbitmq"
	ServiceS3       ServiceType = "s3"
)

type AppService struct {
	ID              uuid.UUID       `json:"id"`
	AppID           uuid.UUID       `json:"app_id"`
	ServiceType     ServiceType     `json:"service_type"`
	DBName          string          `json:"db_name"`
	Credentials     json.RawMessage `json:"-"`           // encrypted
	CredentialsPlain map[string]string `json:"credentials"` // decrypted for response
	CreatedAt       time.Time       `json:"created_at"`
}

type CreateServiceRequest struct {
	ServiceType ServiceType `json:"service_type"`
}
