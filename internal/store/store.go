package store

import (
	"context"
	"encoding/json"
	"fmt"
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
	return &Store{rdb: rdb}
}

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

func (st *Store) ValidateNonce(nonce string) bool {
	key := "nonce:" + nonce
	err := st.rdb.GetDel(ctx, key).Err()
	return err == nil
}

func (st *Store) IsRateLimited(identifier string, limit int) (bool, error) {
	key := "ratelimit:" + identifier

	pipe := st.rdb.Pipeline()
	count := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, 1*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, err
	}

	return count.Val() > int64(limit), nil
}

// Challenge persistence
func (st *Store) SetChallenge(clientIP, nonce string, challenge interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("challenge:%s:%s", clientIP, nonce)
	data, err := json.Marshal(challenge)
	if err != nil {
		return err
	}
	return st.rdb.SetEX(ctx, key, data, ttl).Err()
}

func (st *Store) GetChallenge(clientIP, nonce string, out interface{}) (bool, error) {
	key := fmt.Sprintf("challenge:%s:%s", clientIP, nonce)
	data, err := st.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, err
	}
	return true, nil
}

func (st *Store) DeleteChallenge(clientIP, nonce string) error {
	key := fmt.Sprintf("challenge:%s:%s", clientIP, nonce)
	return st.rdb.Del(ctx, key).Err()
}

// Interactive challenge persistence
func (st *Store) SetInteractiveChallenge(clientIP, nonce string, challenge interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("interactive:%s:%s", clientIP, nonce)
	data, err := json.Marshal(challenge)
	if err != nil {
		return err
	}
	return st.rdb.SetEX(ctx, key, data, ttl).Err()
}

func (st *Store) GetInteractiveChallenge(clientIP, nonce string, out interface{}) (bool, error) {
	key := fmt.Sprintf("interactive:%s:%s", clientIP, nonce)
	data, err := st.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, err
	}
	return true, nil
}

func (st *Store) DeleteInteractiveChallenge(clientIP, nonce string) error {
	key := fmt.Sprintf("interactive:%s:%s", clientIP, nonce)
	return st.rdb.Del(ctx, key).Err()
}

// Offender persistence
func (st *Store) SetOffender(clientIP string, offender interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("offender:%s", clientIP)
	data, err := json.Marshal(offender)
	if err != nil {
		return err
	}
	return st.rdb.SetEX(ctx, key, data, ttl).Err()
}

func (st *Store) GetOffender(clientIP string, out interface{}) (bool, error) {
	key := fmt.Sprintf("offender:%s", clientIP)
	data, err := st.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, err
	}
	return true, nil
}

func (st *Store) DeleteOffender(clientIP string) error {
	key := fmt.Sprintf("offender:%s", clientIP)
	return st.rdb.Del(ctx, key).Err()
}
