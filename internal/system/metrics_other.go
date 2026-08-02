//go:build !linux

package system

import (
	"github.com/anpanel/anpanel/internal/domain"
	"runtime"
	"time"
)

func Snapshot() (domain.HostSnapshot, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return domain.HostSnapshot{Time: time.Now(), MemoryTotal: m.Sys, MemoryUsed: m.Alloc}, nil
}
