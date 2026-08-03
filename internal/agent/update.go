package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
)

// Version is set by main at startup.
var Version = "dev"

const githubRepo = "matthewlu070111/anpanel"

func selfUpdate(ctx context.Context, channel string) (ActionResult, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "stable"
	}
	if channel != "stable" {
		return ActionResult{}, errors.New("only stable updates are supported")
	}
	cfg, err := config.Load()
	if err != nil {
		return ActionResult{}, err
	}
	cfg.UpdateChannel = channel
	if err := config.Save(cfg); err != nil {
		return ActionResult{}, err
	}

	installURL := "https://github.com/" + githubRepo + "/releases/latest/download/install.sh"

	tmpDir, err := os.MkdirTemp("", "anpanel-update-*")
	if err != nil {
		return ActionResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	script := filepath.Join(tmpDir, "install.sh")

	client := &http.Client{Timeout: 3 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, "GET", installURL, nil)
	if err != nil {
		return ActionResult{}, err
	}
	req.Header.Set("User-Agent", "AnPanel/"+Version)
	resp, err := client.Do(req)
	if err != nil {
		return ActionResult{}, fmt.Errorf("download install.sh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ActionResult{}, fmt.Errorf("download install.sh: HTTP %s", resp.Status)
	}
	f, err := os.OpenFile(script, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return ActionResult{}, err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 8<<20)); err != nil {
		f.Close()
		return ActionResult{}, err
	}
	f.Close()

	args := []string{script}
	if cfg.Region == "cn" || cfg.Region == "global" {
		args = append(args, "--region="+cfg.Region)
	}
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Env = append(os.Environ(), "ANPANEL_UPDATE_CHANNEL="+channel)
	b, err := cmd.CombinedOutput()
	out := redact(string(b))
	if err != nil {
		return ActionResult{}, fmt.Errorf("update failed: %s", out)
	}
	return ActionResult{Output: "update completed via " + channel + " channel\n" + out}, nil
}

type updateInfo struct {
	Version        string `json:"version"`
	Channel        string `json:"channel"`
	WebServer      string `json:"web_server"`
	LatestStable   string `json:"latest_stable,omitempty"`
	StableURL      string `json:"stable_url"`
}

func systemInfo(ctx context.Context) updateInfo {
	ws, _ := preferredWebServer()
	info := updateInfo{
		Version:       Version,
		Channel:       "stable",
		WebServer:     ws,
		StableURL:     "https://github.com/" + githubRepo + "/releases/latest",
	}
	// Best-effort latest tags from GitHub API (optional, non-fatal).
	client := &http.Client{Timeout: 8 * time.Second}
	if tag, err := githubLatestTag(ctx, client); err == nil {
		info.LatestStable = tag
	}
	return info
}

func githubLatestTag(ctx context.Context, client *http.Client) (string, error) {
	url := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AnPanel/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("github api %s", resp.Status)
	}
	var v struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.TagName, nil
}
