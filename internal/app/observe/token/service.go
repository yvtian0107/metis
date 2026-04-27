package token

import (
	"errors"
	. "metis/internal/app/observe/domain"
	"sync"
	"time"

	"github.com/samber/do/v2"
)

const maxTokensPerUser = 10

var (
	ErrTokenNotFound     = errors.New("token not found")
	ErrTokenLimitReached = errors.New("token limit reached")
)

// VerifyResult holds resolved identity after a successful token verification.
type VerifyResult struct {
	UserID  uint
	TokenID uint
	Scope   string
}

// cacheEntry holds a cached verify result with expiry.
type cacheEntry struct {
	result    VerifyResult
	expiresAt time.Time
}

type IntegrationTokenService struct {
	repo  *IntegrationTokenRepo
	mu    sync.RWMutex
	cache map[string]cacheEntry // key: raw token string
}

func NewIntegrationTokenService(i do.Injector) (*IntegrationTokenService, error) {
	return &IntegrationTokenService{
		repo:  do.MustInvoke[*IntegrationTokenRepo](i),
		cache: make(map[string]cacheEntry),
	}, nil
}

// Create generates a new integration token for the user.
// Returns the raw plaintext token (display once) and the saved record.
func (s *IntegrationTokenService) Create(userID uint, name string) (string, *IntegrationToken, error) {
	count, err := s.repo.CountByUserID(userID)
	if err != nil {
		return "", nil, err
	}
	if count >= maxTokensPerUser {
		return "", nil, ErrTokenLimitReached
	}

	raw, hash, prefix, err := GenerateIntegrationToken()
	if err != nil {
		return "", nil, err
	}

	t := &IntegrationToken{
		UserID:      userID,
		Scope:       "personal",
		Name:        name,
		TokenHash:   hash,
		TokenPrefix: prefix,
		TokenPlain:  raw,
	}
	if err := s.repo.Create(t); err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

// List returns all active tokens for a user.
func (s *IntegrationTokenService) List(userID uint) ([]IntegrationToken, error) {
	return s.repo.ListByUserID(userID)
}

// Revoke soft-deletes a token and clears it from the verify cache.
func (s *IntegrationTokenService) Revoke(tokenID, userID uint) error {
	t, err := s.repo.FindByIDAndUserID(tokenID, userID)
	if err != nil {
		return ErrTokenNotFound
	}
	if err := s.repo.Revoke(t.ID); err != nil {
		return err
	}
	// Clear cache entries for this token ID (scan by value since key is raw token)
	s.mu.Lock()
	for k, v := range s.cache {
		if v.result.TokenID == tokenID {
			delete(s.cache, k)
		}
	}
	s.mu.Unlock()
	return nil
}

// Verify authenticates a raw token string.
// Uses a 60-second in-memory cache to avoid repeated bcrypt cost.
func (s *IntegrationTokenService) Verify(raw string) (*VerifyResult, error) {
	// Check cache first
	s.mu.RLock()
	if entry, ok := s.cache[raw]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		r := entry.result
		go s.repo.UpdateLastUsed(r.TokenID) //nolint:errcheck
		return &r, nil
	}
	s.mu.RUnlock()

	// Cache miss — do bcrypt lookup
	prefix := ExtractTokenPrefix(raw)
	if prefix == "" {
		return nil, ErrTokenNotFound
	}

	tokens, err := s.repo.FindByPrefix(prefix)
	if err != nil || len(tokens) == 0 {
		return nil, ErrTokenNotFound
	}

	for _, t := range tokens {
		if ValidateIntegrationToken(raw, t.TokenHash) {
			result := VerifyResult{
				UserID:  t.UserID,
				TokenID: t.ID,
				Scope:   t.Scope,
			}
			// Store in cache
			s.mu.Lock()
			s.cache[raw] = cacheEntry{result: result, expiresAt: time.Now().Add(60 * time.Second)}
			s.mu.Unlock()
			go s.repo.UpdateLastUsed(t.ID) //nolint:errcheck
			return &result, nil
		}
	}
	return nil, ErrTokenNotFound
}
