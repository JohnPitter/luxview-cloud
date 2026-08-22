package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
	"github.com/luxview/engine/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var playerUsernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

type Player struct {
	repo *repository.PlayerRepo
}

func NewPlayer(repo *repository.PlayerRepo) *Player {
	return &Player{repo: repo}
}

func ValidatePlayerUsername(username string) error {
	if !playerUsernameRe.MatchString(username) {
		return fmt.Errorf("usuário: 3 a 32 letras, números ou _")
	}
	return nil
}

func ValidatePlayerPassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("a senha precisa de pelo menos 8 caracteres")
	}
	return nil
}

func (p *Player) Register(ctx context.Context, username, password string) (*model.PlayerAccount, error) {
	username = strings.TrimSpace(username)
	if err := ValidatePlayerUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePlayerPassword(password); err != nil {
		return nil, err
	}
	existing, err := p.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("esse usuário já existe")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	acct := &model.PlayerAccount{Username: username, PasswordHash: string(hash)}
	if err := p.repo.Create(ctx, acct); err != nil {
		return nil, err
	}
	if credited, err := p.repo.AppendLedger(ctx, acct.ID, model.LedgerCash, welcomeCash, "welcome"); err == nil {
		acct = credited
	}
	if credited, err := p.repo.AppendLedger(ctx, acct.ID, model.LedgerReward, welcomeReward, "welcome"); err == nil {
		acct = credited
	}
	return acct, nil
}

func (p *Player) Login(ctx context.Context, username, password string) (*model.PlayerAccount, error) {
	acct, err := p.repo.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	if acct == nil || bcrypt.CompareHashAndPassword([]byte(acct.PasswordHash), []byte(password)) != nil {
		return nil, fmt.Errorf("usuário ou senha incorretos")
	}
	return acct, nil
}

func (p *Player) FindByUsername(ctx context.Context, username string) (*model.PlayerAccount, error) {
	acct, err := p.repo.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	if acct == nil {
		return nil, fmt.Errorf("jogador não encontrado")
	}
	return acct, nil
}

func (p *Player) Public(ctx context.Context, acct *model.PlayerAccount) (*model.PlayerPublic, error) {
	links, err := p.repo.ListLinks(ctx, acct.ID)
	if err != nil {
		return nil, err
	}
	return &model.PlayerPublic{
		ID: acct.ID, Username: acct.Username,
		CashPoints: acct.CashPoints, RewardPoints: acct.RewardPoints, Links: links,
	}, nil
}

func (p *Player) Link(ctx context.Context, playerID, appID uuid.UUID, templateID, nick string) (*model.PlayerGameLink, error) {
	nick = strings.TrimSpace(nick)
	if nick == "" || len(nick) > 50 {
		return nil, fmt.Errorf("nick inválido")
	}
	link := &model.PlayerGameLink{
		PlayerID: playerID, AppID: appID, TemplateID: templateID, InGameNick: nick,
	}
	if err := p.repo.CreateLink(ctx, link); err != nil {
		return nil, fmt.Errorf("não foi possível vincular o nick")
	}
	return link, nil
}

func (p *Player) Credit(ctx context.Context, playerID uuid.UUID, kind model.LedgerKind, amount int64, reason string) (*model.PlayerAccount, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("valor inválido")
	}
	return p.repo.AppendLedger(ctx, playerID, kind, amount, reason)
}

func (p *Player) Debit(ctx context.Context, playerID uuid.UUID, kind model.LedgerKind, amount int64, reason string) (*model.PlayerAccount, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("valor inválido")
	}
	return p.repo.AppendLedger(ctx, playerID, kind, -amount, reason)
}
