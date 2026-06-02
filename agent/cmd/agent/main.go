package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ssl-tracker/agent/internal/client"
	"ssl-tracker/agent/internal/config"
	"ssl-tracker/agent/internal/idgen"
	"ssl-tracker/agent/internal/runner"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	hostname, _ := os.Hostname()
	ip := "127.0.0.1"

	agentID, err := idgen.LoadOrCreateID(cfg.Agent.IDFile, hostname, ip)
	if err != nil {
		log.Fatalf("Failed to load/create agent ID: %v", err)
	}
	log.Printf("Agent ID: %s", agentID)

	apiClient := client.NewClient(cfg.ServerURL, cfg.AuthToken)

	if err := apiClient.Register(client.RegisterRequest{
		AgentID:     agentID,
		DisplayName: cfg.Agent.DisplayName,
		Hostname:    hostname,
		IP:          ip,
	}); err != nil {
		log.Fatalf("Failed to register: %v", err)
	}
	log.Println("Registered to server")

	r := runner.NewRunner(apiClient, agentID,
		cfg.Check.IntervalDuration(), cfg.Check.TimeoutDuration(), cfg.Check.Concurrency)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	log.Printf("Agent started, checking every %s", cfg.Check.Interval)
	if err := r.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Runner failed: %v", err)
	}
	log.Println("Agent stopped")
}
