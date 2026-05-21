package grub

import (
	"context"
	"errors"
	"log/slog"
	"os"
)

var ErrConfigNotFound = errors.New("no grub config found in known locations")

var knownConfigPaths = []string{
	"/boot/grub/grub.cfg",
	"/boot/grub2/grub.cfg",
	"/boot/efi/EFI/fedora/grub.cfg",
	"/boot/efi/EFI/redhat/grub.cfg",
	"/boot/efi/EFI/ubuntu/grub.cfg",
}

// DiscoverConfigPath attempts to auto-detect the GRUB config file path.
func (g *Grub) DiscoverConfigPath(ctx context.Context) (string, error) {
	return findConfig()
}

func findConfig() (string, error) {
	for _, path := range knownConfigPaths {
		if _, err := os.Stat(path); err == nil {
			slog.Debug("Found GRUB config at", "path", path)
			return path, nil
		}
	}
	return "", ErrConfigNotFound
}
