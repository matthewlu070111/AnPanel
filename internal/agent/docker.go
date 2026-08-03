package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

func dockerHTTP() *http.Client {
	return &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
	}}, Timeout: 30 * time.Second}
}
func dockerRequest(ctx context.Context, method, path string, out any) error {
	return dockerRequestBody(ctx, method, path, nil, out)
}
func dockerRequestBody(ctx context.Context, method, path string, body any, out any) error {
	var payload *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(b)
	} else {
		payload = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := dockerHTTP().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker API returned %s", resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func dockerObjectAction(ctx context.Context, kind, name, action string) error {
	if !safeID.MatchString(name) {
		return fmt.Errorf("invalid Docker resource name")
	}
	p := url.PathEscape(name)
	switch kind + "." + action {
	case "image.pull":
		return dockerRequest(ctx, "POST", "/images/create?fromImage="+url.QueryEscape(name), nil)
	case "image.delete":
		return dockerRequest(ctx, "DELETE", "/images/"+p+"?force=false", nil)
	case "volume.create":
		return dockerRequestBody(ctx, "POST", "/volumes/create", map[string]string{"Name": name}, nil)
	case "volume.delete":
		return dockerRequest(ctx, "DELETE", "/volumes/"+p+"?force=false", nil)
	case "network.create":
		return dockerRequestBody(ctx, "POST", "/networks/create", map[string]any{"Name": name, "CheckDuplicate": true}, nil)
	case "network.delete":
		return dockerRequest(ctx, "DELETE", "/networks/"+p, nil)
	default:
		return fmt.Errorf("unsupported Docker object action")
	}
}
func dockerContainers(ctx context.Context) ([]domain.Container, error) {
	var raw []struct {
		ID     string   `json:"Id"`
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := dockerRequest(ctx, "GET", "/containers/json?all=true", &raw); err != nil {
		return nil, err
	}
	out := make([]domain.Container, len(raw))
	for i, v := range raw {
		out[i] = domain.Container{ID: v.ID, Names: v.Names, Image: v.Image, State: v.State, Status: v.Status}
	}
	return out, nil
}
func dockerInventory(ctx context.Context, kind string) (any, error) {
	switch kind {
	case "images":
		var v any
		return v, dockerRequest(ctx, "GET", "/images/json?all=true", &v)
	case "networks":
		var v any
		return v, dockerRequest(ctx, "GET", "/networks", &v)
	case "volumes":
		var v any
		return v, dockerRequest(ctx, "GET", "/volumes", &v)
	default:
		return nil, fmt.Errorf("unsupported docker inventory %q", kind)
	}
}
func dockerContainerAction(ctx context.Context, id, action string) error {
	escaped := url.PathEscape(id)
	switch action {
	case "start", "stop", "restart":
		return dockerRequest(ctx, "POST", "/containers/"+escaped+"/"+action, nil)
	case "delete":
		// Running containers return 409 Conflict unless stopped or force-removed.
		_ = dockerRequest(ctx, "POST", "/containers/"+escaped+"/stop?t=10", nil)
		if err := dockerRequest(ctx, "DELETE", "/containers/"+escaped+"?v=false&force=true", nil); err != nil {
			return fmt.Errorf("delete container: %w (若仍失败请确认容器未被其它进程占用)", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported container action")
	}
}
