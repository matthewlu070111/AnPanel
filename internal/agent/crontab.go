package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

func listCrontab(ctx context.Context) ([]domain.CronJob, error) {
	out, err := readCrontab(ctx)
	if err != nil {
		return nil, err
	}
	return parseCrontab(out), nil
}

func readCrontab(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "crontab", "-l")
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		// empty crontab exits 1 with "no crontab"
		if strings.Contains(strings.ToLower(out), "no crontab") || len(strings.TrimSpace(out)) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l failed: %s", redact(out))
	}
	return out, nil
}

func parseCrontab(raw string) []domain.CronJob {
	jobs := []domain.CronJob{}
	i := 0
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		body := trim
		if strings.HasPrefix(body, "@") {
			// @reboot cmd, @daily cmd, ...
			parts := strings.Fields(body)
			if len(parts) < 2 {
				continue
			}
			sched := parts[0]
			cmd := strings.Join(parts[1:], " ")
			i++
			jobs = append(jobs, domain.CronJob{ID: strconv.Itoa(i), Schedule: sched, Command: cmd, Raw: line, Enabled: true})
			continue
		}
		fields := strings.Fields(body)
		if len(fields) < 6 {
			continue
		}
		sched := strings.Join(fields[0:5], " ")
		cmd := strings.Join(fields[5:], " ")
		i++
		jobs = append(jobs, domain.CronJob{ID: strconv.Itoa(i), Schedule: sched, Command: cmd, Raw: line, Enabled: true})
	}
	return jobs
}

func looksLikeCron(s string) bool {
	if strings.HasPrefix(s, "@") {
		return len(strings.Fields(s)) >= 2
	}
	return len(strings.Fields(s)) >= 6
}

func addCrontab(ctx context.Context, schedule, command string) (ActionResult, error) {
	schedule = strings.TrimSpace(schedule)
	command = strings.TrimSpace(command)
	if schedule == "" || command == "" {
		return ActionResult{}, errors.New("schedule and command are required")
	}
	if strings.ContainsAny(command, "\n\r") || strings.ContainsAny(schedule, "\n\r") {
		return ActionResult{}, errors.New("newlines are not allowed")
	}
	line := schedule + " " + command
	if !looksLikeCron(line) {
		return ActionResult{}, errors.New("invalid cron format; use \"m h dom mon dow command\" or \"@daily command\"")
	}
	raw, err := readCrontab(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	if raw != "" && !strings.HasSuffix(raw, "\n") {
		raw += "\n"
	}
	if err := writeCrontab(ctx, raw+line+"\n"); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "cron job added: " + line}, nil
}

func removeCrontab(ctx context.Context, id string) (ActionResult, error) {
	raw, err := readCrontab(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	updated, found := removeCronLine(raw, id)
	if !found {
		return ActionResult{}, errors.New("cron job not found")
	}
	if err := writeCrontab(ctx, updated); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "cron job removed: " + id}, nil
}

func removeCronLine(raw, id string) (string, bool) {
	target, err := strconv.Atoi(id)
	if err != nil || target < 1 {
		return raw, false
	}
	lines, job, found := strings.Split(raw, "\n"), 0, false
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(parseCrontab(line)) > 0 {
			job++
			if job == target {
				found = true
				continue
			}
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n"), found
}

func writeCrontab(ctx context.Context, content string) error {
	cmd := exec.CommandContext(ctx, "crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab install failed: %s", redact(string(out)))
	}
	return nil
}
