package xray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Service wraps Process with application-level state tracking.
// It is the single point through which the app starts, stops, and
// queries the Xray engine.
type Service struct {
	proc         *Process
	state        atomic.Int32 // StateStopped / StateRunning
	mu           sync.Mutex
	trafficMu    sync.Mutex
	binaryPath   string
	apiAddress   string
	tempDir      string
	outboundTags []string
	lastSample   trafficSample
	logHandler   func(string)
}

type TrafficStats struct {
	UploadBytes            uint64 `json:"uploadBytes"`
	DownloadBytes          uint64 `json:"downloadBytes"`
	UploadBytesPerSecond   uint64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond uint64 `json:"downloadBytesPerSecond"`
}

type trafficSample struct {
	upload   uint64
	download uint64
	at       time.Time
}

// NewService creates a service that manages one Xray instance.
func NewService() *Service {
	s := &Service{proc: NewProcess()}
	return s
}

func (s *Service) SetLogHandler(handler func(string)) {
	s.mu.Lock()
	s.logHandler = handler
	s.mu.Unlock()
}

// Start begins the Xray process using the binary at binaryPath with the
// given config file. It returns a typed error on failure.
func (s *Service) Start(binaryPath, configPath string) error {
	return s.start(binaryPath, configPath, "", "", nil)
}

// StartWithConfig instruments a pasted Xray JSON config for traffic stats and
// stores the private working copy outside the project directory.
func (s *Service) StartWithConfig(binaryPath, rawConfig string) error {
	outboundTags := configuredOutboundTags(rawConfig)
	return s.startWithConfig(binaryPath, rawConfig, outboundTags)
}

func (s *Service) startWithConfig(binaryPath, rawConfig string, outboundTags []string) error {
	if binaryPath == "" {
		var err error
		binaryPath, err = DefaultBinaryPath()
		if err != nil {
			return err
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("xray: reserve stats API address: %w", err)
	}
	apiAddress := listener.Addr().String()
	_ = listener.Close()

	config, err := instrumentConfig(rawConfig, apiAddress)
	if err != nil {
		return err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("xray: locate cache directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(cacheDir, "packet-drift-xray-")
	if err != nil {
		return fmt.Errorf("xray: create private config directory: %w", err)
	}
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("xray: write private config: %w", err)
	}
	validateCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.CommandContext(validateCtx, binaryPath, "run", "-test", "-c", configPath).CombinedOutput()
	cancel()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("xray: validate config: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := s.start(binaryPath, configPath, apiAddress, tempDir, outboundTags); err != nil {
		_ = os.RemoveAll(tempDir)
		return err
	}
	return nil
}

// StartWithURI converts a supported proxy share URI to an Xray config.
func (s *Service) StartWithURI(binaryPath, proxyURI, mode, proxyAddress string) error {
	config, err := configFromURIWithOptions(proxyURI, mode, proxyAddress)
	if err != nil {
		return err
	}
	return s.startWithConfig(binaryPath, config, configuredOutboundTags(config))
}

func (s *Service) start(binaryPath, configPath, apiAddress, tempDir string, outboundTags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc.IsRunning() {
		return ErrAlreadyRunning
	}
	if err := s.proc.Start(binaryPath, configPath, s.logHandler); err != nil {
		return err
	}
	s.binaryPath = binaryPath
	s.apiAddress = apiAddress
	s.tempDir = tempDir
	s.outboundTags = append([]string(nil), outboundTags...)
	s.lastSample = trafficSample{}
	s.state.Store(int32(StateRunning))
	return nil
}

// Stop terminates the Xray process and waits for it to exit.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.proc.IsRunning() {
		return ErrNotRunning
	}
	err := s.proc.Stop()
	if err == nil {
		s.state.Store(int32(StateStopped))
		s.clearRuntimeLocked()
	}
	return err
}

// State reports the current lifecycle state.
func (s *Service) State() State {
	if s.proc.IsRunning() {
		return StateRunning
	}
	s.state.Store(int32(StateStopped))
	return StateStopped
}

// Shutdown is a best-effort Stop used on application exit.
// It never returns an error so it is safe to call unconditionally.
func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc.IsRunning() {
		_ = s.proc.Stop()
	}
	s.state.Store(int32(StateStopped))
	s.clearRuntimeLocked()
}

// Traffic returns cumulative outbound bytes and rates since the previous call.
func (s *Service) Traffic() (TrafficStats, error) {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	s.mu.Lock()
	if !s.proc.IsRunning() {
		s.mu.Unlock()
		return TrafficStats{}, nil
	}
	binaryPath, apiAddress := s.binaryPath, s.apiAddress
	outboundTags := append([]string(nil), s.outboundTags...)
	s.mu.Unlock()
	if apiAddress == "" {
		return TrafficStats{}, ErrStatsUnavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binaryPath, "api", "statsquery", "-server", apiAddress, "-pattern", "outbound>>>").CombinedOutput()
	if err != nil {
		return TrafficStats{}, fmt.Errorf("xray: query traffic stats: %w: %s", err, strings.TrimSpace(string(output)))
	}
	upload, download := parseTrafficCountersForTags(string(output), outboundTags)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.proc.IsRunning() || s.apiAddress != apiAddress {
		return TrafficStats{}, nil
	}
	result := TrafficStats{UploadBytes: upload, DownloadBytes: download}
	if !s.lastSample.at.IsZero() {
		elapsed := now.Sub(s.lastSample.at)
		if elapsed > 0 && upload >= s.lastSample.upload && download >= s.lastSample.download {
			result.UploadBytesPerSecond = uint64(float64(upload-s.lastSample.upload) / elapsed.Seconds())
			result.DownloadBytesPerSecond = uint64(float64(download-s.lastSample.download) / elapsed.Seconds())
		}
	}
	s.lastSample = trafficSample{upload: upload, download: download, at: now}
	return result, nil
}

func (s *Service) clearRuntimeLocked() {
	if s.tempDir != "" {
		_ = os.RemoveAll(s.tempDir)
	}
	s.binaryPath = ""
	s.apiAddress = ""
	s.tempDir = ""
	s.outboundTags = nil
	s.lastSample = trafficSample{}
}

func parseTrafficCounters(output string) (upload, download uint64) {
	return parseTrafficCountersForTags(output, nil)
}

func parseTrafficCountersForTags(output string, tags []string) (upload, download uint64) {
	var response struct {
		Stats []struct {
			Name  string `json:"name"`
			Value uint64 `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal([]byte(output), &response); err == nil {
		for _, stat := range response.Stats {
			if !isTrackedOutboundStat(stat.Name, tags) {
				continue
			}
			switch {
			case strings.HasSuffix(stat.Name, ">>>traffic>>>uplink"):
				upload += stat.Value
			case strings.HasSuffix(stat.Name, ">>>traffic>>>downlink"):
				download += stat.Value
			}
		}
		return upload, download
	}
	return upload, download
}

func isTrackedOutboundStat(name string, tags []string) bool {
	if !strings.HasPrefix(name, "outbound>>>") {
		return false
	}
	if len(tags) == 0 {
		return true
	}
	for _, tag := range tags {
		prefix := "outbound>>>" + tag + ">>>traffic>>>"
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func configuredOutboundTags(rawConfig string) []string {
	var config struct {
		Outbounds []struct {
			Tag string `json:"tag"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return nil
	}
	tags := make([]string, 0, len(config.Outbounds))
	for _, outbound := range config.Outbounds {
		if outbound.Tag != "" {
			tags = append(tags, outbound.Tag)
		}
	}
	return tags
}

func instrumentConfig(rawConfig, apiAddress string) ([]byte, error) {
	decoder := json.NewDecoder(strings.NewReader(rawConfig))
	decoder.UseNumber()
	var config map[string]any
	if err := decoder.Decode(&config); err != nil || config == nil {
		if err == nil {
			err = errors.New("top-level JSON value must be an object")
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	api, err := objectField(config, "api")
	if err != nil {
		return nil, err
	}
	tag, _ := api["tag"].(string)
	if tag == "" {
		tag = "packet-drift-api"
		api["tag"] = tag
	}
	api["listen"] = apiAddress
	services, ok := api["services"].([]any)
	if !ok {
		services = []any{}
	}
	hasStats := false
	for _, service := range services {
		if service == "StatsService" {
			hasStats = true
			break
		}
	}
	if !hasStats {
		services = append(services, "StatsService")
	}
	api["services"] = services
	config["api"] = api

	stats, err := objectField(config, "stats")
	if err != nil {
		return nil, err
	}
	config["stats"] = stats

	policy, err := objectField(config, "policy")
	if err != nil {
		return nil, err
	}
	system, err := nestedObject(policy, "system", "policy")
	if err != nil {
		return nil, err
	}
	system["statsOutboundUplink"] = true
	system["statsOutboundDownlink"] = true
	config["policy"] = policy

	result, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	return result, nil
}

func objectField(parent map[string]any, key string) (map[string]any, error) {
	if value, ok := parent[key]; ok {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: %s must be an object", ErrInvalidConfig, key)
		}
		return object, nil
	}
	object := make(map[string]any)
	parent[key] = object
	return object, nil
}

func nestedObject(parent map[string]any, key, parentName string) (map[string]any, error) {
	if value, ok := parent[key]; ok {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: %s.%s must be an object", ErrInvalidConfig, parentName, key)
		}
		return object, nil
	}
	object := make(map[string]any)
	parent[key] = object
	return object, nil
}
