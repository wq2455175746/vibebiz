package usertrace

import (
	"fmt"
	"os"
)

func EnsureWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create user trace dir: %w", err)
	}
	probe, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("probe user trace dir: %w", err)
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("close user trace probe: %w", err)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("remove user trace probe: %w", err)
	}
	return nil
}
