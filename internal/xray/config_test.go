package xray

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestInstrumentConfigPreservesAndEnablesStats(t *testing.T) {
	input := `{"api":{"tag":"custom-api","services":["HandlerService"]},"policy":{"system":{"statsInboundUplink":false}},"outbounds":[{"tag":"proxy"}]}`
	got, err := instrumentConfig(input, "127.0.0.1:12345")
	if err != nil {
		t.Fatalf("instrumentConfig(): %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(got, &config); err != nil {
		t.Fatalf("decode instrumented config: %v", err)
	}
	api := config["api"].(map[string]any)
	if api["tag"] != "custom-api" || api["listen"] != "127.0.0.1:12345" {
		t.Fatalf("api settings = %#v", api)
	}
	services := api["services"].([]any)
	if len(services) != 2 || services[0] != "HandlerService" || services[1] != "StatsService" {
		t.Fatalf("api services = %#v", services)
	}
	policy := config["policy"].(map[string]any)
	system := policy["system"].(map[string]any)
	if system["statsInboundUplink"] != false || system["statsOutboundUplink"] != true || system["statsOutboundDownlink"] != true {
		t.Fatalf("system policy = %#v", system)
	}
	if _, ok := config["stats"].(map[string]any); !ok {
		t.Fatalf("stats field missing or invalid: %#v", config["stats"])
	}
}

func TestInstrumentConfigRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", `[]`, `{invalid}`, `{} {}`} {
		if _, err := instrumentConfig(input, "127.0.0.1:12345"); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("instrumentConfig(%q) error = %v, want ErrInvalidConfig", input, err)
		}
	}
}

func TestParseTrafficCounters(t *testing.T) {
	output := `{"stat":[{"name":"outbound>>>proxy>>>traffic>>>uplink","value":1200},{"name":"outbound>>>direct>>>traffic>>>uplink","value":300},{"name":"outbound>>>proxy>>>traffic>>>downlink","value":4500},{"name":"other>>>traffic>>>uplink","value":99}]}`
	upload, download := parseTrafficCounters(output)
	if upload != 1500 || download != 4500 {
		t.Fatalf("parseTrafficCounters() = (%d, %d), want (1500, 4500)", upload, download)
	}
}

func TestParseTrafficCountersForConfiguredOutbounds(t *testing.T) {
	output := `{"stat":[{"name":"outbound>>>proxy>>>traffic>>>uplink","value":1200},{"name":"outbound>>>proxy>>>traffic>>>downlink","value":4500},{"name":"outbound>>>proxy-backup>>>traffic>>>uplink","value":300},{"name":"inbound>>>local-socks>>>traffic>>>uplink","value":999}]}`
	upload, download := parseTrafficCountersForTags(output, []string{"proxy"})
	if upload != 1200 || download != 4500 {
		t.Fatalf("parseTrafficCountersForTags() = (%d, %d), want (1200, 4500)", upload, download)
	}
}
