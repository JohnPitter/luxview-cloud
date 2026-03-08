package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/luxview/engine/pkg/crypto"
)

type SettingsRepo struct {
	db            *DB
	encryptionKey []byte
}

func NewSettingsRepo(db *DB, encryptionKey []byte) *SettingsRepo {
	return &SettingsRepo{db: db, encryptionKey: encryptionKey}
}

// Get retrieves a setting by key, decrypting if necessary.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	var encrypted bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT value, encrypted FROM platform_settings WHERE key = $1`, key,
	).Scan(&value, &encrypted)
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}

	if encrypted {
		decrypted, err := crypto.Decrypt(value, r.encryptionKey)
		if err != nil {
			return "", fmt.Errorf("decrypt setting %q: %w", key, err)
		}
		return decrypted, nil
	}

	return value, nil
}

// Set creates or updates a setting, encrypting the value if requested.
func (r *SettingsRepo) Set(ctx context.Context, key, value string, encrypted bool) error {
	storeValue := value
	if encrypted {
		enc, err := crypto.Encrypt(value, r.encryptionKey)
		if err != nil {
			return fmt.Errorf("encrypt setting %q: %w", key, err)
		}
		storeValue = enc
	}

	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO platform_settings (key, value, encrypted, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, encrypted = $3, updated_at = NOW()`,
		key, storeValue, encrypted,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// GetAll retrieves all settings matching the given prefix, decrypting as needed.
func (r *SettingsRepo) GetAll(ctx context.Context, prefix string) (map[string]string, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT key, value, encrypted FROM platform_settings WHERE key LIKE $1`,
		prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("get all settings with prefix %q: %w", prefix, err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		var encrypted bool
		if err := rows.Scan(&key, &value, &encrypted); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}

		if encrypted {
			decrypted, err := crypto.Decrypt(value, r.encryptionKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt setting %q: %w", key, err)
			}
			value = decrypted
		}

		result[strings.TrimPrefix(key, prefix)] = value
	}
	return result, nil
}

// Delete removes a setting by key.
func (r *SettingsRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM platform_settings WHERE key = $1`, key,
	)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
