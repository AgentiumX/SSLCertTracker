package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	serverURL string
	token     string
	http      *http.Client
}

func NewClient(serverURL, token string) *Client {
	return &Client{
		serverURL: serverURL,
		token:     token,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

type RegisterRequest struct {
	AgentID     string `json:"agent_id"`
	DisplayName string `json:"display_name"`
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
}

type Domain struct {
	ID       uint   `json:"id"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type CheckResult struct {
	DomainID     uint       `json:"domain_id"`
	CheckedAt    time.Time  `json:"checked_at"`
	Status       string     `json:"status"`
	NotAfter     *time.Time `json:"not_after"`
	Issuer       string     `json:"issuer"`
	Subject      string     `json:"subject"`
	SANs         string     `json:"sans"`
	ErrorMessage string     `json:"error_message"`
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.serverURL+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) Register(req RegisterRequest) error {
	resp, err := c.do("POST", "/api/agent/register", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register failed: %d %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) GetDomains(agentID string) ([]Domain, error) {
	resp, err := c.do("GET", "/api/agent/domains?agent_id="+agentID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get domains failed: %d %s", resp.StatusCode, b)
	}
	var result struct {
		Domains []Domain `json:"domains"`
	}
	return result.Domains, json.NewDecoder(resp.Body).Decode(&result)
}

func (c *Client) PostResults(agentID string, results []CheckResult) error {
	resp, err := c.do("POST", "/api/agent/results", map[string]interface{}{
		"agent_id": agentID,
		"results":  results,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post results failed: %d %s", resp.StatusCode, b)
	}
	return nil
}
