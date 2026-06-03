package store

import (
	"testing"

	"gorm.io/gorm"
)

func setupUserTestDB(t *testing.T) *Store {
	s := setupTestDB(t)
	if err := s.db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateUser(t *testing.T) {
	s := setupUserTestDB(t)
	u := &User{Username: "admin", PasswordHash: "$2a$10$hash"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if u.ID == 0 {
		t.Errorf("expected ID > 0 after create")
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	s := setupUserTestDB(t)
	if err := s.CreateUser(&User{Username: "admin", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	err := s.CreateUser(&User{Username: "admin", PasswordHash: "h2"})
	if err == nil {
		t.Errorf("expected duplicate username error, got nil")
	}
}

func TestGetUserByUsername_Found(t *testing.T) {
	s := setupUserTestDB(t)
	if err := s.CreateUser(&User{Username: "admin", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("expected admin, got %s", got.Username)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := setupUserTestDB(t)
	_, err := s.GetUserByUsername("nope")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestCountUsers(t *testing.T) {
	s := setupUserTestDB(t)
	n, err := s.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	if err := s.CreateUser(&User{Username: "a", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUser(&User{Username: "b", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	n, err = s.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}
