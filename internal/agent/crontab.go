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
	cmd := exec.CommandContext(ctx, "crontab", "-l")
	b, err := cmd.CombinedOutput()
	out := string(b)
	if err != nil {
		// empty crontab exits 1 with "no crontab"
		if strings.Contains(strings.ToLower(out), "no crontab") || len(strings.TrimSpace(out)) == 0 {
			return []domain.CronJob{}, nil
		}
		return nil, fmt.Errorf("crontab -l failed: %s", redact(out))
	}
	return parseCrontab(out), nil
}

func parseCrontab(raw string) []domain.CronJob {
	jobs := []domain.CronJob{}
	i := 0
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		enabled := true
		body := trim
		if strings.HasPrefix(body, "#") {
			// disabled job if looks like a cron line after #
			rest := strings.TrimSpace(strings.TrimPrefix(body, "#"))
			if !looksLikeCron(rest) {
				continue // plain comment
			}
			enabled = false
			body = rest
		}
		if strings.HasPrefix(body, "@") {
			// @reboot cmd, @daily cmd, ...
			parts := strings.Fields(body)
			if len(parts) < 2 {
				continue
			}
			sched := parts[0]
			cmd := strings.Join(parts[1:], " ")
			i++
			jobs = append(jobs, domain.CronJob{ID: strconv.Itoa(i), Schedule: sched, Command: cmd, Raw: line, Enabled: enabled})
			continue
		}
		fields := strings.Fields(body)
		if len(fields) < 6 {
			continue
		}
		sched := strings.Join(fields[0:5], " ")
		cmd := strings.Join(fields[5:], " ")
		i++
		jobs = append(jobs, domain.CronJob{ID: strconv.Itoa(i), Schedule: sched, Command: cmd, Raw: line, Enabled: enabled})
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
	jobs, err := listCrontab(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	// rebuild file
	var b strings.Builder
	for _, j := range jobs {
		if j.Enabled {
			b.WriteString(j.Schedule + " " + j.Command + "\n")
		} else {
			b.WriteString("# " + j.Schedule + " " + j.Command + "\n")
		}
	}
	b.WriteString(line + "\n")
	if err := writeCrontab(ctx, b.String()); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "cron job added: " + line}, nil
}

func removeCrontab(ctx context.Context, id string) (ActionResult, error) {
	jobs, err := listCrontab(ctx)
	if err != nil {
		return ActionResult{}, err
	}
	found := false
	var b strings.Builder
	for _, j := range jobs {
		if j.ID == id {
			found = true
			continue
		}
		if j.Enabled {
			b.WriteString(j.Schedule + " " + j.Command + "\n")
		} else {
			b.WriteString("# " + j.Schedule + " " + j.Command + "\n")
		}
	}
	if !found {
		return ActionResult{}, errors.New("cron job not found")
	}
	if err := writeCrontab(ctx, b.String()); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "cron job removed: " + id}, nil
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
