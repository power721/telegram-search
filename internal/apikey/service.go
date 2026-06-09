package apikey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

const settingsSecretKey = "api_key.encryption_secret"
const mediaSigningContext = "tg-search-media-signing"

type Service struct {
	keys     *repository.APIKeyRepository
	settings *repository.SettingsRepository
}

func NewService(keys *repository.APIKeyRepository, settings *repository.SettingsRepository) *Service {
	return &Service{keys: keys, settings: settings}
}

func (s *Service) EnsureActive(ctx context.Context) (model.APIKeyResponse, error) {
	active, err := s.keys.Active(ctx)
	if err == nil && active.KeyCiphertext != "" {
		return s.response(ctx, active)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.APIKeyResponse{}, err
	}
	return s.Regenerate(ctx)
}

func (s *Service) Active(ctx context.Context) (model.APIKeyResponse, error) {
	active, err := s.keys.Active(ctx)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	if active.KeyCiphertext == "" {
		return s.Regenerate(ctx)
	}
	return s.response(ctx, active)
}

func (s *Service) Regenerate(ctx context.Context) (model.APIKeyResponse, error) {
	plaintext, err := newPlaintext()
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return model.APIKeyResponse{}, fmt.Errorf("hash api key: %w", err)
	}
	ciphertext, err := s.encrypt(ctx, plaintext)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	if err := s.keys.DisableEnabled(ctx); err != nil {
		return model.APIKeyResponse{}, err
	}
	id, err := s.keys.Create(ctx, model.APIKey{
		Name:          "default",
		KeyHash:       string(hash),
		KeyCiphertext: ciphertext,
		Prefix:        plaintext[:8],
		Enabled:       true,
	})
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	active, err := s.keys.Active(ctx)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	active.ID = id
	return toResponse(active, plaintext), nil
}

func (s *Service) Verify(ctx context.Context, plaintext string) (int64, bool, error) {
	if len(plaintext) < 8 {
		return 0, false, nil
	}
	candidates, err := s.keys.EnabledByPrefix(ctx, plaintext[:8])
	if err != nil {
		return 0, false, err
	}
	for _, candidate := range candidates {
		if bcrypt.CompareHashAndPassword([]byte(candidate.KeyHash), []byte(plaintext)) == nil {
			if err := s.keys.RecordUsage(ctx, candidate.ID, time.Now().UTC()); err != nil {
				return 0, false, err
			}
			return candidate.ID, true, nil
		}
	}
	return 0, false, nil
}

func (s *Service) VerifyMediaSignature(ctx context.Context, method string, path string, exp string, sig string, now time.Time) (bool, error) {
	expiresAt, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || expiresAt <= now.Unix() {
		return false, nil
	}
	active, err := s.keys.Active(ctx)
	if err != nil {
		return false, err
	}
	plaintext, err := s.decrypt(ctx, active.KeyCiphertext)
	if err != nil {
		return false, err
	}
	expected, err := MediaSignature(plaintext, method, path, exp)
	if err != nil {
		return false, err
	}
	expectedBytes, _ := hex.DecodeString(expected)
	gotBytes, err := hex.DecodeString(sig)
	if err != nil {
		return false, nil
	}
	if !hmac.Equal(gotBytes, expectedBytes) {
		return false, nil
	}
	if err := s.keys.RecordUsage(ctx, active.ID, now.UTC()); err != nil {
		return false, err
	}
	return true, nil
}

func MediaSignature(apiKey string, method string, path string, exp string) (string, error) {
	if apiKey == "" || method == "" || path == "" || exp == "" {
		return "", fmt.Errorf("media signature inputs are required")
	}
	derive := hmac.New(sha256.New, []byte(apiKey))
	if _, err := derive.Write([]byte(mediaSigningContext)); err != nil {
		return "", err
	}
	signingKey := derive.Sum(nil)
	mac := hmac.New(sha256.New, signingKey)
	if _, err := mac.Write([]byte(method + "\n" + path + "\n" + exp)); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) response(ctx context.Context, key model.APIKey) (model.APIKeyResponse, error) {
	plaintext, err := s.decrypt(ctx, key.KeyCiphertext)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	return toResponse(key, plaintext), nil
}

func toResponse(key model.APIKey, plaintext string) model.APIKeyResponse {
	return model.APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.Prefix,
		Key:        plaintext,
		UsageCount: key.UsageCount,
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
	}
}

func newPlaintext() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) encrypt(ctx context.Context, plaintext string) (string, error) {
	aead, err := s.aead(ctx)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate api key nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Service) decrypt(ctx context.Context, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode api key ciphertext: %w", err)
	}
	aead, err := s.aead(ctx)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("api key ciphertext is too short")
	}
	nonce := raw[:aead.NonceSize()]
	data := raw[aead.NonceSize():]
	opened, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	return string(opened), nil
}

func (s *Service) aead(ctx context.Context) (cipher.AEAD, error) {
	secret, ok, err := s.settings.Get(ctx, settingsSecretKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("generate api key secret: %w", err)
		}
		secret = hex.EncodeToString(raw)
		if err := s.settings.Set(ctx, settingsSecretKey, secret); err != nil {
			return nil, err
		}
	}
	key, err := hex.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("decode api key secret: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create api key cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
