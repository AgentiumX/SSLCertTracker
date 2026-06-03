package main

import (
	"flag"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/api"
	"ssl-tracker/server/internal/config"
	"ssl-tracker/server/internal/store"
	"ssl-tracker/server/internal/web"
)

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
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	s := store.NewStore(db)
	r := api.SetupRouter(s, cfg.Auth.AgentToken, cfg.Alert.ExpireThresholdDays, web.Handler())

	log.Printf("Server starting on %s", cfg.Server.Listen)
	if err := r.Run(cfg.Server.Listen); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
