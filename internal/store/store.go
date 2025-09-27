package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var ctx = context.Background()

type Session struct {
	VerifiedAt              time.Time `json:"verifiedAt"`
	LastSeen                time.Time `json:"lastSeen"`
	HasScrolled             bool      `json:"hasScrolled"`
	HasNaturalMouseMovement bool      `json:"hasNaturalMouseMovement"`
	PagesViewed             int       `json:"pagesViewed"`
	NavigationPath          []string  `json:"navigationPath"`
}

type Store struct {
	rdb *redis.Client
}

func New(redisAddr string) *Store {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	// We no longer need the background cleanup goroutine for visitors.
	return &Store{rdb: rdb}
}

// --- Session Methods (No changes here) ---

func (st *Store) GetSession(token string) (*Session, bool) {
	key := "session:" + token
	val, err := st.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var session Session
	if err := json.Unmarshal(val, &session); err != nil {
		return nil, false
	}
	return &session, true
}

func (st *Store) SetSession(token string, session *Session, timeout time.Duration) error {
	key := "session:" + token
	val, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return st.rdb.Set(ctx, key, val, timeout).Err()
}

func (st *Store) DeleteSession(token string) error {
	key := "session:" + token
	return st.rdb.Del(ctx, key).Err()
}

func (st *Store) CreateNonce(ttl time.Duration) (string, error) {
	nonce := uuid.New().String()
	key := "nonce:" + nonce
	err := st.rdb.Set(ctx, key, "valid", ttl).Err()
	return nonce, err
}

// ValidateNonce checks if a nonce exists and immediately deletes it to prevent reuse.
func (st *Store) ValidateNonce(nonce string) bool {
	key := "nonce:" + nonce
	// GETDEL is an atomic get-and-delete operation.
	// If the key exists, it's deleted and its value is returned. Otherwise, it returns an error.
	err := st.rdb.GetDel(ctx, key).Err()
	// If there's no error, the key existed and was deleted successfully.
	return err == nil
}

// --- NEW: Rate Limiter Method using Redis ---

// IsRateLimited checks if an identifier has exceeded a limit in the last minute.
func (st *Store) IsRateLimited(identifier string, limit int) (bool, error) {
	key := "ratelimit:" + identifier

	// Use a pipeline to execute commands atomically and efficiently.
	pipe := st.rdb.Pipeline()
	// INCR returns the new value of the key after incrementing.
	count := pipe.Incr(ctx, key)
	// Set the key to expire in 1 minute, but only if it's a new key.
	pipe.ExpireNX(ctx, key, 1*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, err // Fail closed (assume rate limited on error)
	}

	// Check if the count for this minute has exceeded the limit.
	return count.Val() > int64(limit), nil
}
