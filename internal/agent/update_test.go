package agent

import (
	"context"
	"strings"
	"testing"
)

func TestSelfUpdateRejectsPrerelease(t *testing.T) {
	_, err := selfUpdate(context.Background(), "prerelease")
	if err == nil || !strings.Contains(err.Error(), "only stable") {
		t.Fatalf("expected prerelease to be rejected, got %v", err)
	}
}
