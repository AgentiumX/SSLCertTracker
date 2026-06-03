package auth

import (
	"sync"
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore(time.Hour)
	sid := store.Create(42, "admin")
	if len(sid) != 64 {
		t.Errorf("expected 64-char session id, got %d", len(sid))
	}
	sess, ok := store.Get(sid)
	if !ok {
		t.Fatalf("expected to find created session")
	}
	if sess.UserID != 42 || sess.Username != "admin" {
		t.Errorf("unexpected session payload: %+v", sess)
	}
}

func TestSessionStore_GetUnknown(t *testing.T) {
	store := NewSessionStore(time.Hour)
	if _, ok := store.Get("does-not-exist"); ok {
		t.Errorf("expected ok=false for unknown session id")
	}
}

func TestSessionStore_Expired(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	sid := store.Create(1, "u")
	time.Sleep(20 * time.Millisecond)
	if _, ok := store.Get(sid); ok {
		t.Errorf("expired session should not be returned")
	}
}

func TestSessionStore_SlidingExpiry(t *testing.T) {
	store := NewSessionStore(100 * time.Millisecond)
	sid := store.Create(1, "u")
	time.Sleep(60 * time.Millisecond)
	// Get should refresh ExpiresAt
	if _, ok := store.Get(sid); !ok {
		t.Fatalf("session should still be valid")
	}
	time.Sleep(60 * time.Millisecond) // total 120ms since create, but only 60ms since last Get
	if _, ok := store.Get(sid); !ok {
		t.Errorf("sliding expiry should have kept session alive")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore(time.Hour)
	sid := store.Create(1, "u")
	store.Delete(sid)
	if _, ok := store.Get(sid); ok {
		t.Errorf("deleted session should not be retrievable")
	}
}

func TestSessionStore_Delete_Unknown(t *testing.T) {
	store := NewSessionStore(time.Hour)
	store.Delete("does-not-exist") // must not panic
}

func TestSessionStore_Cleanup(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	sid1 := store.Create(1, "u1")
	time.Sleep(20 * time.Millisecond)
	sid2 := store.Create(2, "u2")
	store.Cleanup()
	// sid1 expired, should be gone after Cleanup
	if _, ok := store.peekRaw(sid1); ok {
		t.Errorf("expected sid1 to be cleaned up")
	}
	// sid2 still fresh
	if _, ok := store.peekRaw(sid2); !ok {
		t.Errorf("expected sid2 to remain")
	}
}

func TestSessionStore_Concurrent(t *testing.T) {
	store := NewSessionStore(time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sid := store.Create(uint(i), "u")
			_, _ = store.Get(sid)
			store.Delete(sid)
		}(i)
	}
	wg.Wait()
}

func TestSessionStore_TTL(t *testing.T) {
	store := NewSessionStore(time.Hour)
	if store.TTL() != time.Hour {
		t.Errorf("expected TTL=1h, got %v", store.TTL())
	}
}
