//go:build linux

package config

import (
	"errors"
	"os"
	"syscall"
)

func preserveOwnership(target, reference string) error {
	st, err := os.Stat(reference)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	raw, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	return os.Chown(target, int(raw.Uid), int(raw.Gid))
}
