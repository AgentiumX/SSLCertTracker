package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/api"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/config"
	"ssl-tracker/server/internal/store"
	"ssl-tracker/server/internal/web"
)

const defaultSessionTTL = 24 * time.Hour

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var db *gorm.DB
	switch cfg.Database.Type {
	case "sqlite":
		if err := os.MkdirAll("data", 0755); err != nil {
			log.Fatalf("Failed to create data dir: %v", err)
		}
		db, err = gorm.Open(sqlite.Open(cfg.Database.SQLite.Path), &gorm.Config{})
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.Database.MySQL.DSN), &gorm.Config{})
	default:
		log.Fatalf("Unsupported database type: %s", cfg.Database.Type)
	}
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&store.Agent{},
		&store.Domain{},
		&store.AgentDomainOverride{},
		&store.CheckResult{},
		&store.User{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	s := store.NewStore(db)

	if err := bootstrapAdminUser(s, cfg); err != nil {
		log.Fatalf("Failed to bootstrap admin user: %v", err)
	}

	ttl := defaultSessionTTL
	if cfg.Session.TTL != "" {
		parsed, err := time.ParseDuration(cfg.Session.TTL)
		if err != nil {
			log.Fatalf("Invalid session.ttl %q: %v", cfg.Session.TTL, err)
		}
		ttl = parsed
	}
	sessions := auth.NewSessionStore(ttl)
	go runSessionCleanup(sessions)

	r := api.SetupRouter(s, cfg.Auth.AgentToken, cfg.Alert.ExpireThresholdDays,
		sessions, cfg.Session.Secure, web.Handler())

	log.Printf("Server starting on %s", cfg.Server.Listen)
	if err := r.Run(cfg.Server.Listen); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func bootstrapAdminUser(s *store.Store, cfg *config.Config) error {
	count, err := s.CountUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("users table has %d users, skipping initial admin creation", count)
		return nil
	}
	if cfg.Auth.AdminUsername == "" || cfg.Auth.AdminPassword == "" {
		log.Fatal("users table is empty and auth.admin_username/admin_password is not configured")
	}
	hash, err := auth.HashPassword(cfg.Auth.AdminPassword)
	if err != nil {
		return err
	}
	if err := s.CreateUser(&store.User{Username: cfg.Auth.AdminUsername, PasswordHash: hash}); err != nil {
		return err
	}
	log.Printf("created initial admin user: %s", cfg.Auth.AdminUsername)
	return nil
}

func runSessionCleanup(sessions *auth.SessionStore) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sessions.Cleanup()
	}
}
