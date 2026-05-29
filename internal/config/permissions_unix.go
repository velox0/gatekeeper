//go:build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

func ensureConfigFileSecure(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	mode := info.Mode().Perm()
	if mode != 0o600 {
		return fmt.Errorf("config file %s must have permissions 0600 (found %04o)", path, mode)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("config file %s: unable to read ownership metadata", path)
	}

	uid := uint32(os.Geteuid())
	if stat.Uid != uid {
		return fmt.Errorf("config file %s must be owned by uid %d", path, uid)
	}

	return nil
}
