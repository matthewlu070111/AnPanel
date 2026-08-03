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
	"regexp"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
)

// Version is set by main at startup.
var Version = "dev"

const githubRepo = "matthewlu070111/anpanel"

var versionInNotes = regexp.MustCompile(`(?i)AnPanel\s+(v?[A-Za-z0-9][A-Za-z0-9._+-]{0,63})`)
var versionInTitle = regexp.MustCompile(`\(([A-Za-z0-9][A-Za-z0-9._+-]{0,63})\)`)
var versionLine = regexp.MustCompile(`(?m)^VERSION=([A-Za-z0-9][A-Za-z0-9._+-]{0,63})\s*$`)
var installVersionLine = regexp.MustCompile(`(?m)^TARGET_VERSION=\$\{ANPANEL_VERSION:-(.+)\}$`)

func selfUpdate(ctx context.Context, channel string) (ActionResult, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "stable"
	}
	installURL, err := updateInstallURL(channel)
	if err != nil {
		return ActionResult{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return ActionResult{}, err
	}
	cfg.UpdateChannel = channel
	if err := config.Save(cfg); err != nil {
		return ActionResult{}, err
	}

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
	// Install script should already restart units; schedule again so even older
	// install.sh builds still load the new binary after self_update returns.
	schedulePanelRestart()
	return ActionResult{Output: "update completed via " + channel + " channel; restarting anpanel-agent and anpanel-web\n" + out}, nil
}

// schedulePanelRestart reloads both panel units after a short delay so the
// current action can finish writing its task result first.
func schedulePanelRestart() {
	go func() {
		time.Sleep(2 * time.Second)
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "restart", "anpanel-agent.service", "anpanel-web.service").Run()
	}()
}

func updateInstallURL(channel string) (string, error) {
	switch channel {
	case "stable":
		return "https://github.com/" + githubRepo + "/releases/latest/download/install.sh", nil
	case "prerelease":
		return "https://github.com/" + githubRepo + "/releases/download/prerelease-latest/install.sh", nil
	default:
		return "", errors.New("channel must be stable or prerelease")
	}
}

type updateInfo struct {
	Version           string `json:"version"`
	Channel           string `json:"channel"`
	WebServer         string `json:"web_server"`
	LatestStable      string `json:"latest_stable,omitempty"`
	LatestPrerelease  string `json:"latest_prerelease,omitempty"`
	UpdateAvailable   bool   `json:"update_available"`
	StableURL         string `json:"stable_url"`
	PrereleaseURL     string `json:"prerelease_url"`
}

func systemInfo(ctx context.Context) updateInfo {
	cfg, _ := config.Load()
	channel := cfg.UpdateChannel
	if channel != "prerelease" {
		channel = "stable"
	}
	ws, _ := preferredWebServer()
	info := updateInfo{
		Version:       Version,
		Channel:       channel,
		WebServer:     ws,
		StableURL:     "https://github.com/" + githubRepo + "/releases/latest",
		PrereleaseURL: "https://github.com/" + githubRepo + "/releases/tag/prerelease-latest",
	}
	client := &http.Client{Timeout: 10 * time.Second}
	if tag, err := githubLatestTag(ctx, client); err == nil {
		info.LatestStable = tag
	}
	if tag, err := githubPrereleaseLatestVersion(ctx, client); err == nil {
		info.LatestPrerelease = tag
	}
	// Fallback: parse embedded version from prerelease-latest install.sh
	if info.LatestPrerelease == "" {
		if tag, err := prereleaseVersionFromInstallScript(ctx, client); err == nil {
			info.LatestPrerelease = tag
		}
	}
	target := info.LatestStable
	if channel == "prerelease" {
		target = info.LatestPrerelease
	}
	info.UpdateAvailable = target != "" && !versionEqual(Version, target)
	return info
}

func versionEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
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
		Name    string `json:"name"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	if v.TagName != "" {
		return v.TagName, nil
	}
	return "", errors.New("empty tag")
}

// githubPrereleaseLatestVersion reads the moving prerelease-latest alias notes/title.
func githubPrereleaseLatestVersion(ctx context.Context, client *http.Client) (string, error) {
	url := "https://api.github.com/repos/" + githubRepo + "/releases/tags/prerelease-latest"
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
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	// Preferred: VERSION=build-abc1234 in release body
	if m := versionLine.FindStringSubmatch(v.Body); len(m) > 1 {
		return m[1], nil
	}
	// Title: "Latest AnPanel prerelease (build-abc1234)"
	if m := versionInTitle.FindStringSubmatch(v.Name); len(m) > 1 {
		return m[1], nil
	}
	// Notes: "This moving alias installs AnPanel build-abc1234. ..."
	if m := versionInNotes.FindStringSubmatch(v.Body); len(m) > 1 {
		return m[1], nil
	}
	return "", errors.New("prerelease version not found in release notes")
}

func prereleaseVersionFromInstallScript(ctx context.Context, client *http.Client) (string, error) {
	url := "https://github.com/" + githubRepo + "/releases/download/prerelease-latest/install.sh"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AnPanel/"+Version)
	// Only need the first few KB for TARGET_VERSION line.
	req.Header.Set("Range", "bytes=0-4095")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return "", fmt.Errorf("download install.sh: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if m := installVersionLine.FindSubmatch(b); len(m) > 1 {
		v := string(m[1])
		if v != "" && v != "latest" {
			return v, nil
		}
	}
	return "", errors.New("version not embedded in install.sh")
}
