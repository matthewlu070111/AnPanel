package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
	"github.com/matthewlu070111/anpanel/internal/domain"
)

type Client struct {
	http   *http.Client
	token  string
	socket string
}

func NewClient(cfg config.Config) (*Client, error) {
	b, err := os.ReadFile(cfg.AgentTokenFile)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", cfg.AgentSocket)
	}}
	// Short timeout for reads; Action uses a longer client below.
	return &Client{http: &http.Client{Transport: tr, Timeout: 30 * time.Second}, token: string(bytes.TrimSpace(b)), socket: cfg.AgentSocket}, nil
}
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(c.http, ctx, "GET", path, nil, out)
}
func (c *Client) Action(ctx context.Context, a ActionRequest) (ActionResult, error) {
	var out ActionResult
	// Certificate issuance / renew / package install can take several minutes.
	actionHTTP := &http.Client{Transport: c.http.Transport, Timeout: 12 * time.Minute}
	err := c.do(actionHTTP, ctx, "POST", "/v1/action", a, &out)
	return out, err
}
func (c *Client) do(cli *http.Client, ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("agent: %s: %s", resp.Status, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
func (c *Client) Snapshot(ctx context.Context) (domain.HostSnapshot, error) {
	var m domain.HostSnapshot
	err := c.Get(ctx, "/v1/metrics", &m)
	return m, err
}
