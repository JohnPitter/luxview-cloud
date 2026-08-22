package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	CommunityChatMax      = 280
	CommunityTitleMax     = 80
	CommunityBodyMax      = 2000
	communityChatCooldown = 2 * time.Second
	communityPresenceTTL  = 45 * time.Second
)

type communitySeat struct {
	username string
	seen     time.Time
}

type CommunityHub struct {
	mu       sync.Mutex
	lastChat map[uuid.UUID]time.Time
	here     map[uuid.UUID]communitySeat
}

func NewCommunityHub() *CommunityHub {
	return &CommunityHub{
		lastChat: map[uuid.UUID]time.Time{},
		here:     map[uuid.UUID]communitySeat{},
	}
}

func NormalizeCommunityText(s string, max int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("texto vazio")
	}
	if utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("texto longo demais")
	}
	return s, nil
}

func (h *CommunityHub) AllowChat(playerID uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if last, ok := h.lastChat[playerID]; ok && time.Since(last) < communityChatCooldown {
		return fmt.Errorf("aguarde um instante para enviar de novo")
	}
	h.lastChat[playerID] = time.Now()
	return nil
}

func (h *CommunityHub) Touch(playerID uuid.UUID, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.here[playerID] = communitySeat{username: username, seen: time.Now()}
}

func (h *CommunityHub) OnlineCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	cutoff := time.Now().Add(-communityPresenceTTL)
	for id, seat := range h.here {
		if seat.seen.Before(cutoff) {
			delete(h.here, id)
			continue
		}
		n++
	}
	return n
}
