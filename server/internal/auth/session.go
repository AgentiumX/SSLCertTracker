package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	UserID    uint
	Username  string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

func (s *SessionStore) TTL() time.Duration {
	return s.ttl
}

// Create generates a new session and returns the session id (64 hex chars).
func (s *SessionStore) Create(userID uint, username string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on a healthy system never fails; if it does we
		// cannot generate a session safely.
		panic("crypto/rand failed: " + err.Error())
	}
	sid := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[sid] = &Session{
		UserID:    userID,
		Username:  username,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return sid
}

// Get returns the session if it exists and has not expired. On a hit it also
// performs sliding expiry: ExpiresAt is reset to now+ttl.
func (s *SessionStore) Get(sid string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sid]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, sid)
		return nil, false
	}
	sess.ExpiresAt = time.Now().Add(s.ttl)
	// Return a copy so callers can't mutate internal state.
	cp := *sess
	return &cp, true
}

// peekRaw returns whether a session exists in the map, without expiry checks
// or sliding renewal. Test-only helper.
func (s *SessionStore) peekRaw(sid string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sid]
	return sess, ok
}

func (s *SessionStore) Delete(sid string) {
	s.mu.Lock()
	delete(s.sessions, sid)
	s.mu.Unlock()
}

// Cleanup removes all expired sessions.
func (s *SessionStore) Cleanup() {
	now := time.Now()
	s.mu.Lock()
	for sid, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, sid)
		}
	}
	s.mu.Unlock()
}
