package xray

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed assets/*/*
var bundledAssets embed.FS

const (
	bundledAssetsDirectory = "assets"
	xrayCacheDirectory     = "packet-drift"
	xrayVersion            = "xray-v26.3.27"
	xrayBinaryName         = "xray"
	xrayWindowsBinaryName  = "xray.exe"
)

// DefaultBinaryPath extracts the embedded Xray assets for the current
// platform into the user's cache and returns the executable path.
func DefaultBinaryPath() (string, error) {
	platform, binaryName, err := currentPlatform()
	if err != nil {
		return "", err
	}

	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("xray: locate cache directory: %w", err)
	}

	targetDirectory := filepath.Join(
		cacheDirectory,
		xrayCacheDirectory,
		xrayVersion,
		platform,
	)

	if err := os.MkdirAll(targetDirectory, 0o700); err != nil {
		return "", fmt.Errorf("xray: create cache directory: %w", err)
	}

	assetDirectory := filepath.ToSlash(filepath.Join(bundledAssetsDirectory, platform))
	entries, err := fs.ReadDir(bundledAssets, assetDirectory)
	if err != nil {
		return "", fmt.Errorf("xray: read embedded assets for %s: %w", platform, err)
	}

	binaryFound := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		assetName := entry.Name()
		sourcePath := filepath.ToSlash(filepath.Join(assetDirectory, assetName))
		targetPath := filepath.Join(targetDirectory, assetName)

		data, err := fs.ReadFile(bundledAssets, sourcePath)
		if err != nil {
			return "", fmt.Errorf("xray: read embedded asset %s: %w", assetName, err)
		}

		if assetName == binaryName {
			binaryFound = true
		}

		if err := writeAssetIfChanged(targetPath, data, assetName == binaryName); err != nil {
			return "", fmt.Errorf("xray: extract asset %s: %w", assetName, err)
		}
	}

	if !binaryFound {
		return "", fmt.Errorf("xray: embedded executable %q is missing for %s", binaryName, platform)
	}

	return filepath.Join(targetDirectory, binaryName), nil
}

// currentPlatform maps the current Go target to its embedded asset directory.
func currentPlatform() (platform, binaryName string, err error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64":
		return "darwin-amd64", xrayBinaryName, nil
	case "darwin/arm64":
		return "darwin-arm64", xrayBinaryName, nil
	case "linux/amd64":
		return "linux-amd64", xrayBinaryName, nil
	case "linux/arm64":
		return "linux-arm64", xrayBinaryName, nil
	case "windows/amd64":
		return "windows-amd64", xrayWindowsBinaryName, nil
	case "windows/arm64":
		return "windows-arm64", xrayWindowsBinaryName, nil
	default:
		return "", "", fmt.Errorf(
			"xray: unsupported platform %s/%s",
			runtime.GOOS,
			runtime.GOARCH,
		)
	}
}

// writeAssetIfChanged replaces a cached asset only when its contents differ.
func writeAssetIfChanged(targetPath string, data []byte, executable bool) error {
	existingData, err := os.ReadFile(targetPath)
	if err == nil && bytes.Equal(existingData, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read cached asset: %w", err)
	}

	fileMode := os.FileMode(0o644)
	if executable {
		fileMode = 0o755
	}

	temporaryFile, err := os.CreateTemp(filepath.Dir(targetPath), ".xray-asset-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if _, err := temporaryFile.Write(data); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}

	if err := temporaryFile.Chmod(fileMode); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("set file permissions: %w", err)
	}

	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	// Windows does not replace an existing destination with os.Rename.
	if runtime.GOOS == "windows" {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old cached asset: %w", err)
		}
	}

	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return fmt.Errorf("replace cached asset: %w", err)
	}

	return nil
}
