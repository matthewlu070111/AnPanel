//go:build linux

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anpanel/anpanel/internal/domain"
)

type cpuSample struct{ idle, total uint64 }

var previousCPU cpuSample

func Snapshot() (domain.HostSnapshot, error) {
	m := domain.HostSnapshot{Time: time.Now()}
	cur := readCPU()
	if previousCPU.total > 0 && cur.total > previousCPU.total {
		dt := cur.total - previousCPU.total
		di := cur.idle - previousCPU.idle
		m.CPUPercent = 100 * float64(dt-di) / float64(dt)
	}
	previousCPU = cur
	load, _ := os.ReadFile("/proc/loadavg")
	if f := strings.Fields(string(load)); len(f) > 0 {
		m.Load1, _ = strconv.ParseFloat(f[0], 64)
	}
	mem := readKV("/proc/meminfo")
	m.MemoryTotal = mem["MemTotal"] * 1024
	m.MemoryUsed = (mem["MemTotal"] - mem["MemAvailable"]) * 1024
	m.SwapTotal = mem["SwapTotal"] * 1024
	m.SwapUsed = (mem["SwapTotal"] - mem["SwapFree"]) * 1024
	up, _ := os.ReadFile("/proc/uptime")
	if f := strings.Fields(string(up)); len(f) > 0 {
		v, _ := strconv.ParseFloat(f[0], 64)
		m.Uptime = uint64(v)
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil {
		m.DiskTotal = st.Blocks * uint64(st.Bsize)
		m.DiskUsed = (st.Blocks - st.Bavail) * uint64(st.Bsize)
	}
	f, _ := os.Open("/proc/net/dev")
	if f != nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			parts := strings.Fields(strings.ReplaceAll(s.Text(), ":", " "))
			if len(parts) >= 10 && parts[0] != "lo" {
				rx, _ := strconv.ParseUint(parts[1], 10, 64)
				tx, _ := strconv.ParseUint(parts[9], 10, 64)
				m.NetRX += rx
				m.NetTX += tx
			}
		}
	}
	return m, nil
}
func readCPU() cpuSample {
	b, _ := os.ReadFile("/proc/stat")
	f := strings.Fields(string(b))
	var c cpuSample
	if len(f) >= 8 && f[0] == "cpu" {
		for i := 1; i < len(f) && i <= 8; i++ {
			v, _ := strconv.ParseUint(f[i], 10, 64)
			c.total += v
			if i == 4 || i == 5 {
				c.idle += v
			}
		}
	}
	return c
}
func readKV(path string) map[string]uint64 {
	out := map[string]uint64{}
	f, _ := os.Open(path)
	if f == nil {
		return out
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		p := strings.Fields(s.Text())
		if len(p) >= 2 {
			k := strings.TrimSuffix(p[0], ":")
			out[k], _ = strconv.ParseUint(p[1], 10, 64)
		}
	}
	return out
}
