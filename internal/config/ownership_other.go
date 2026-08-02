//go:build !linux

package config

func preserveOwnership(target, reference string) error { return nil }
