package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// UploadFile streams file bytes to the agent upload endpoint.
func (c *Client) UploadFile(ctx context.Context, dir, name string, overwrite bool, body io.Reader, size int64) error {
	q := url.Values{}
	q.Set("path", dir)
	q.Set("name", name)
	if overwrite {
		q.Set("overwrite", "1")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/files/upload?"+q.Encode(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/octet-stream")
	if size > 0 {
		req.ContentLength = size
	}
	cli := &http.Client{Transport: c.http.Transport, Timeout: 15 * time.Minute}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("agent: %s: %s", resp.Status, string(b))
	}
	return nil
}

// OpenDownload opens a streaming download from the agent. Caller must close the body.
func (c *Client) OpenDownload(ctx context.Context, path string) (body io.ReadCloser, contentLength int64, filename string, err error) {
	q := url.Values{}
	q.Set("path", path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/v1/files/download?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	cli := &http.Client{Transport: c.http.Transport, Timeout: 15 * time.Minute}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("agent: %s: %s", resp.Status, string(b))
	}
	filename = pathBaseName(path)
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if i := strings.Index(cd, "filename="); i >= 0 {
			filename = strings.Trim(cd[i+9:], `"`)
		}
	}
	return resp.Body, resp.ContentLength, filename, nil
}

func pathBaseName(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}
