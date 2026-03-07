package model

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type User struct {
	ID          uuid.UUID  `json:"id"`
	GitHubID    int64      `json:"github_id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	AvatarURL   string     `json:"avatar_url"`
	GitHubToken string     `json:"-"` // never exposed in JSON
	Role        UserRole   `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	PlanID      *uuid.UUID `json:"plan_id,omitempty"`
	Plan        *Plan      `json:"plan,omitempty"`
}

type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	AvatarURL string     `json:"avatar_url"`
	Role      UserRole   `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	PlanID    *uuid.UUID `json:"plan_id,omitempty"`
	Plan      *Plan      `json:"plan,omitempty"`
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		PlanID:    u.PlanID,
		Plan:      u.Plan,
	}
}
