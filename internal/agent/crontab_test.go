package agent

import (
	"strings"
	"testing"
)

func TestCrontabCommentsAreIgnoredAndPreserved(t *testing.T) {
	raw := "# backup notes\n# 0 1 * * * /old-job\n0 2 * * * /active-job\n"
	jobs := parseCrontab(raw)
	if len(jobs) != 1 || jobs[0].Command != "/active-job" {
		t.Fatalf("unexpected jobs: %#v", jobs)
	}
	updated, found := removeCronLine(raw, "1")
	if !found || !strings.Contains(updated, "# backup notes") || strings.Contains(updated, "/active-job") {
		t.Fatalf("comments not preserved: %q", updated)
	}
}
