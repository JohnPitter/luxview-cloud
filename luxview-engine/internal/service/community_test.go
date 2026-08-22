package service_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/service"
)

func TestNormalizeCommunityText(t *testing.T) {
	if _, err := service.NormalizeCommunityText("  ", 10); err == nil {
		t.Fatal("empty text should fail")
	}
	got, err := service.NormalizeCommunityText("  oi  ", 10)
	if err != nil || got != "oi" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := service.NormalizeCommunityText("abcdefghijk", 10); err == nil {
		t.Fatal("over-limit should fail")
	}
}

func TestCommunityHubChatCooldown(t *testing.T) {
	hub := service.NewCommunityHub()
	id := uuid.New()
	if err := hub.AllowChat(id); err != nil {
		t.Fatal(err)
	}
	if err := hub.AllowChat(id); err == nil {
		t.Fatal("expected cooldown")
	}
	time.Sleep(10 * time.Millisecond)
}

func TestCommunityHubPresence(t *testing.T) {
	hub := service.NewCommunityHub()
	hub.Touch(uuid.New(), "joao")
	if n := hub.OnlineCount(); n != 1 {
		t.Fatalf("online=%d", n)
	}
}
