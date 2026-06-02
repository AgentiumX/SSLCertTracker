package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ssl-tracker/agent/internal/checker"
	"ssl-tracker/agent/internal/client"
)

// pickFreePort returns a TCP port that is currently free.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	// agent/e2e → ../..
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// startServerBinary builds and starts the server binary against an isolated config
// and returns its base URL plus a cleanup function.
func startServerBinary(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()

	tmp := t.TempDir()
	port := pickFreePort(t)
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := fmt.Sprintf(`server:
  listen: "127.0.0.1:%d"
auth:
  agent_token: "e2e-token"
  admin_username: "admin"
  admin_password: "admin"
database:
  type: sqlite
  sqlite:
    path: "%s"
retention:
  history_days: 7
alert:
  expire_threshold_days: 15
  daily_reminder_time: "09:00"
  daily_reminder_timezone: "Asia/Shanghai"
session:
  secret: "e2e-secret"
  ttl: "24h"
`, port, filepath.ToSlash(filepath.Join(dataDir, "e2e.db")))

	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(tmp, "server.exe")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	build.Dir = filepath.Join(repoRoot(t), "server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath, "-config", cfgPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait until server is ready
	deadline := time.Now().Add(15 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/admin/domains")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		cmd.Process.Kill()
		t.Fatal("server did not become ready in time")
	}

	cleanup = func() {
		cmd.Process.Kill()
		cmd.Wait()
	}
	return baseURL, cleanup
}

func adminCreateDomain(t *testing.T, baseURL string, payload map[string]interface{}) uint {
	t.Helper()
	body, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/api/admin/domains", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create domain failed: %d %s", resp.StatusCode, b)
	}
	var r struct {
		ID uint `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.ID
}

func adminListDomains(t *testing.T, baseURL string) []map[string]interface{} {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/admin/domains")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r struct {
		Domains []map[string]interface{} `json:"domains"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Domains
}

func TestE2E_FullCycle(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("SKIP_E2E set")
	}

	baseURL, cleanup := startServerBinary(t)
	defer cleanup()

	id := adminCreateDomain(t, baseURL, map[string]interface{}{
		"host": "example.com", "port": 443, "protocol": "https", "is_global": true,
	})
	if id == 0 {
		t.Fatal("expected domain ID > 0")
	}

	domains := adminListDomains(t, baseURL)
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}

	c := client.NewClient(baseURL, "e2e-token")
	if err := c.Register(client.RegisterRequest{
		AgentID: "e2e-001", DisplayName: "E2E", Hostname: "h", IP: "127.0.0.1",
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := c.GetDomains("e2e-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 domain for agent, got %d", len(got))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cr := checker.CheckDomain(ctx, got[0].Host, got[0].Port, got[0].Protocol)
	if cr.Status == "unreachable" {
		t.Skipf("Network issue: %s", cr.ErrorMessage)
	}

	sansJSON, _ := json.Marshal(cr.SANs)
	if err := c.PostResults("e2e-001", []client.CheckResult{{
		DomainID: got[0].ID, CheckedAt: time.Now(),
		Status: cr.Status, NotAfter: cr.NotAfter,
		Issuer: cr.Issuer, Subject: cr.Subject, SANs: string(sansJSON),
	}}); err != nil {
		t.Fatalf("PostResults failed: %v", err)
	}

	t.Logf("E2E full cycle pass: status=%s issuer=%s", cr.Status, cr.Issuer)
}

func TestE2E_TokenRequired(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("SKIP_E2E set")
	}

	baseURL, cleanup := startServerBinary(t)
	defer cleanup()

	c := client.NewClient(baseURL, "wrong-token")
	err := c.Register(client.RegisterRequest{AgentID: "x", DisplayName: "x"})
	if err == nil {
		t.Fatal("expected register to fail with wrong token")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}
