package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/luxview/engine/internal/model"
)

type CommunityRepo struct {
	db *DB
}

func NewCommunityRepo(db *DB) *CommunityRepo {
	return &CommunityRepo{db: db}
}

func (r *CommunityRepo) CreatePost(ctx context.Context, post *model.CommunityPost, authorID uuid.UUID) error {
	return r.db.Pool.QueryRow(ctx,
		`INSERT INTO community_posts (app_id, author_user_id, title, body)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		post.AppID, authorID, post.Title, post.Body,
	).Scan(&post.ID, &post.CreatedAt)
}

func (r *CommunityRepo) DeletePost(ctx context.Context, postID, appID uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM community_posts WHERE id = $1 AND app_id = $2`, postID, appID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}
	return nil
}

func (r *CommunityRepo) ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]model.CommunityPost, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT p.id, p.app_id, p.title, p.body, p.created_at
		 FROM community_posts p
		 WHERE p.app_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}

func (r *CommunityRepo) ListFeed(ctx context.Context, limit int) ([]model.CommunityPost, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT p.id, p.app_id, COALESCE(c.template_id, ''), a.name, p.title, p.body, p.created_at
		 FROM community_posts p
		 JOIN apps a ON a.id = p.app_id
		 LEFT JOIN game_server_configs c ON c.app_id = p.app_id
		 ORDER BY p.created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.CommunityPost
	for rows.Next() {
		var p model.CommunityPost
		if err := rows.Scan(&p.ID, &p.AppID, &p.Game, &p.GameName, &p.Title, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.CommunityPost{}
	}
	return out, rows.Err()
}

func (r *CommunityRepo) InsertChat(ctx context.Context, playerID uuid.UUID, username, body string) (*model.CommunityMessage, error) {
	msg := &model.CommunityMessage{Username: username, Body: body}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO community_chat_messages (player_id, username, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		playerID, username, body,
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (r *CommunityRepo) ListChat(ctx context.Context, limit int) ([]model.CommunityMessage, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, username, body, created_at
		 FROM community_chat_messages
		 ORDER BY created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var newestFirst []model.CommunityMessage
	for rows.Next() {
		var m model.CommunityMessage
		if err := rows.Scan(&m.ID, &m.Username, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		newestFirst = append(newestFirst, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]model.CommunityMessage, len(newestFirst))
	for i, m := range newestFirst {
		out[len(newestFirst)-1-i] = m
	}
	if out == nil {
		out = []model.CommunityMessage{}
	}
	return out, nil
}

func scanPosts(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.CommunityPost, error) {
	var out []model.CommunityPost
	for rows.Next() {
		var p model.CommunityPost
		if err := rows.Scan(&p.ID, &p.AppID, &p.Title, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.CommunityPost{}
	}
	return out, rows.Err()
}
