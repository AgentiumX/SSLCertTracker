package runner

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"ssl-tracker/agent/internal/checker"
	"ssl-tracker/agent/internal/client"
)

type Runner struct {
	client      *client.Client
	agentID     string
	interval    time.Duration
	timeout     time.Duration
	concurrency int64
}

func NewRunner(c *client.Client, agentID string, interval, timeout time.Duration, concurrency int) *Runner {
	return &Runner{client: c, agentID: agentID, interval: interval, timeout: timeout, concurrency: int64(concurrency)}
}

func (r *Runner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	if err := r.runOnce(ctx); err != nil {
		log.Printf("Initial check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.runOnce(ctx); err != nil {
				log.Printf("Check cycle failed: %v", err)
			}
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) error {
	domains, err := r.client.GetDomains(r.agentID)
	if err != nil {
		return err
	}
	if len(domains) == 0 {
		log.Println("No domains to check")
		return nil
	}
	log.Printf("Checking %d domains...", len(domains))

	sem := semaphore.NewWeighted(r.concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]client.CheckResult, 0, len(domains))

	for _, d := range domains {
		domain := d
		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer sem.Release(1)
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("Panic checking domain %d: %v", domain.ID, rec)
					mu.Lock()
					results = append(results, client.CheckResult{
						DomainID: domain.ID, CheckedAt: time.Now(),
						Status: "unreachable", ErrorMessage: "panic during check",
					})
					mu.Unlock()
				}
			}()

			checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
			defer cancel()
			cr := checker.CheckDomain(checkCtx, domain.Host, domain.Port, domain.Protocol)
			sansJSON, _ := json.Marshal(cr.SANs)

			mu.Lock()
			results = append(results, client.CheckResult{
				DomainID: domain.ID, CheckedAt: time.Now(),
				Status: cr.Status, NotAfter: cr.NotAfter,
				Issuer: cr.Issuer, Subject: cr.Subject, SANs: string(sansJSON),
				ErrorMessage: cr.ErrorMessage,
			})
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(results) > 0 {
		if err := r.client.PostResults(r.agentID, results); err != nil {
			return err
		}
		log.Printf("Reported %d results", len(results))
	}
	return nil
}
