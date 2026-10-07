package xray

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed assets/linux-amd64/*
var bundledAssets embed.FS

func DefaultBinaryPath() (string, error) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("xray: bundled binary is only available for linux/amd64")
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("xray: locate cache directory: %w", err)
	}
	targetDir := filepath.Join(cacheDir, "packet-drift", "xray-v26.3.27", "linux-amd64")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return "", fmt.Errorf("xray: create binary cache: %w", err)
	}
	entries, err := fs.ReadDir(bundledAssets, "assets/linux-amd64")
	if err != nil {
		return "", err
	}
	complete := true
	for _, entry := range entries {
		source := filepath.ToSlash(filepath.Join("assets/linux-amd64", entry.Name()))
		info, err := fs.Stat(bundledAssets, source)
		if err != nil {
			return "", err
		}
		targetInfo, err := os.Stat(filepath.Join(targetDir, entry.Name()))
		if err != nil || targetInfo.Size() != info.Size() {
			complete = false
			break
		}
	}
	if !complete {
		for _, entry := range entries {
			source := filepath.ToSlash(filepath.Join("assets/linux-amd64", entry.Name()))
			data, err := fs.ReadFile(bundledAssets, source)
			if err != nil {
				return "", err
			}
			mode := os.FileMode(0o644)
			if entry.Name() == "xray" {
				mode = 0o755
			}
			tmp, err := os.CreateTemp(targetDir, ".xray-asset-*")
			if err != nil {
				return "", err
			}
			if _, err = tmp.Write(data); err == nil {
				err = tmp.Chmod(mode)
			}
			if closeErr := tmp.Close(); err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(tmp.Name(), filepath.Join(targetDir, entry.Name()))
			}
			if err != nil {
				_ = os.Remove(tmp.Name())
				return "", fmt.Errorf("xray: extract bundled %s: %w", entry.Name(), err)
			}
		}
	}
	return filepath.Join(targetDir, "xray"), nil
}
