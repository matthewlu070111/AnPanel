package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var safeDockerImage = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,200}$`)

// deployDockerAndBind pulls/runs a container and optionally creates a reverse-proxy site.
func deployDockerAndBind(ctx context.Context, opts map[string]string) (ActionResult, error) {
	image := strings.TrimSpace(opts["image"])
	name := strings.TrimSpace(opts["name"])
	if image == "" {
		return ActionResult{}, errors.New("image is required")
	}
	if !safeDockerImage.MatchString(image) {
		return ActionResult{}, errors.New("invalid image name")
	}
	if name == "" {
		base := strings.ReplaceAll(image, "/", "-")
		base = strings.ReplaceAll(base, ":", "-")
		name = "anpanel-" + domainSlug(base)
		if len(name) > 40 {
			name = name[:40]
		}
	}
	if !safeID.MatchString(name) {
		return ActionResult{}, errors.New("invalid container name")
	}
	hostPort := strings.TrimSpace(opts["host_port"])
	containerPort := strings.TrimSpace(opts["container_port"])
	if hostPort == "" {
		hostPort = "8080"
	}
	if containerPort == "" {
		containerPort = "80"
	}
	if _, err := strconv.Atoi(hostPort); err != nil {
		return ActionResult{}, errors.New("invalid host_port")
	}
	if _, err := strconv.Atoi(containerPort); err != nil {
		return ActionResult{}, errors.New("invalid container_port")
	}

	if _, err := run(ctx, "docker", "pull", image); err != nil {
		return ActionResult{}, fmt.Errorf("pull image: %w", err)
	}

	args := []string{"run", "-d", "--name", name, "--restart", "unless-stopped", "-p", hostPort + ":" + containerPort}
	if env := strings.TrimSpace(opts["env"]); env != "" {
		for _, pair := range strings.Split(env, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			if !strings.Contains(pair, "=") || strings.ContainsAny(pair, " \t\n\r") {
				return ActionResult{}, errors.New("invalid env entry; use KEY=VAL,KEY2=VAL2 without spaces")
			}
			args = append(args, "-e", pair)
		}
	}
	args = append(args, image)
	res, err := run(ctx, "docker", args...)
	if err != nil {
		return ActionResult{}, err
	}
	out := "container started: " + name + "\n" + res.Output

	domain := normalizedDomain(opts["domain"])
	if domain != "" {
		if !domainName.MatchString(domain) {
			return ActionResult{Output: out + "\nwarning: invalid domain, site not created"}, nil
		}
		siteOpts := map[string]string{
			"domain":     domain,
			"site_type":  "proxy",
			"proxy_pass": "http://127.0.0.1:" + hostPort,
			"enable_ssl": opts["enable_ssl"],
			"tool":       opts["tool"],
			"email":      opts["email"],
			"server":     opts["server"],
		}
		siteRes, siteErr := createWebsite(ctx, siteOpts)
		if siteErr != nil {
			out += "\ncontainer running, but site bind failed: " + siteErr.Error()
			return ActionResult{Output: out}, nil
		}
		out += "\n" + siteRes.Output
	}
	return ActionResult{Output: out}, nil
}
