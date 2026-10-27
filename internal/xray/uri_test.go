package xray

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfigFromURI(t *testing.T) {
	vmessProfile := `{"ps":"vmess-test","add":"vmess.example","port":"8443","id":"11111111-1111-4111-8111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example","path":"/edge","tls":"tls","sni":"cdn.example"}`
	encodedVMess := base64.StdEncoding.EncodeToString([]byte(vmessProfile))
	encodedShadowsocks := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret@ss.example:8388"))
	tests := []struct {
		name     string
		uri      string
		protocol string
		tag      string
	}{
		{
			name:     "vless reality websocket",
			uri:      "vless://11111111-1111-4111-8111-111111111111@vless.example:443?encryption=none&security=reality&sni=example.com&fp=chrome&pbk=public-key&sid=abcd&type=xhttp&host=cdn.example&path=%2Fedge#vless-test",
			protocol: "vless",
			tag:      "vless-test",
		},
		{
			name:     "vmess websocket",
			uri:      "vmess://" + encodedVMess,
			protocol: "vmess",
			tag:      "vmess-test",
		},
		{
			name:     "trojan tls",
			uri:      "trojan://secret@trojan.example:443?sni=example.com&type=grpc&serviceName=proxy#trojan-test",
			protocol: "trojan",
			tag:      "trojan-test",
		},
		{
			name:     "shadowsocks SIP002",
			uri:      "ss://aes-128-gcm:secret@ss.example:8388#ss-test",
			protocol: "shadowsocks",
			tag:      "ss-test",
		},
		{
			name:     "shadowsocks base64",
			uri:      "ss://" + encodedShadowsocks,
			protocol: "shadowsocks",
			tag:      "proxy",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generated, err := configFromURI(test.uri)
			if err != nil {
				t.Fatalf("configFromURI(): %v", err)
			}
			var config map[string]any
			if err := json.Unmarshal([]byte(generated), &config); err != nil {
				t.Fatalf("decode generated config: %v", err)
			}
			inbounds := config["inbounds"].([]any)
			inbound := inbounds[0].(map[string]any)
			if inbound["protocol"] != "socks" || inbound["listen"] != "127.0.0.1" || inbound["port"] != float64(10808) {
				t.Fatalf("generated inbound = %#v", inbound)
			}
			outbounds := config["outbounds"].([]any)
			outbound := outbounds[0].(map[string]any)
			if outbound["protocol"] != test.protocol || outbound["tag"] != test.tag {
				t.Fatalf("generated outbound = %#v", outbound)
			}
		})
	}
}

func TestConfigFromURIMapsStreamSettings(t *testing.T) {
	generated, err := configFromURI("vless://user@example.com:443?security=reality&sni=site.example&fp=chrome&pbk=key&sid=1234&spx=%2F&type=xhttp&host=cdn.example&path=%2Fsocket")
	if err != nil {
		t.Fatalf("configFromURI(): %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(generated), &config); err != nil {
		t.Fatalf("decode generated config: %v", err)
	}
	outbound := config["outbounds"].([]any)[0].(map[string]any)
	stream := outbound["streamSettings"].(map[string]any)
	if stream["network"] != "xhttp" || stream["security"] != "reality" {
		t.Fatalf("stream settings = %#v", stream)
	}
	reality := stream["realitySettings"].(map[string]any)
	if reality["serverName"] != "site.example" || reality["publicKey"] != "key" || reality["shortId"] != "1234" {
		t.Fatalf("reality settings = %#v", reality)
	}
	xhttp := stream["xhttpSettings"].(map[string]any)
	if xhttp["path"] != "/socket" || xhttp["host"] != "cdn.example" {
		t.Fatalf("xhttp settings = %#v", xhttp)
	}
}

func TestConfigFromURIMapsVMessRawNetworkToTCP(t *testing.T) {
	profile := `{"ps":"raw-node","add":"server.example","port":9090,"id":"11111111-1111-4111-8111-111111111111","aid":0,"scy":"auto","net":"raw"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(profile))
	generated, err := configFromURI(uri)
	if err != nil {
		t.Fatalf("configFromURI(): %v", err)
	}
	var config struct {
		Outbounds []struct {
			StreamSettings struct {
				Network     string `json:"network"`
				TCPSettings struct {
					Header struct {
						Type string `json:"type"`
					} `json:"header"`
				} `json:"tcpSettings"`
			} `json:"streamSettings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(generated), &config); err != nil {
		t.Fatalf("decode generated config: %v", err)
	}
	if len(config.Outbounds) != 1 {
		t.Fatalf("outbounds = %#v, want one outbound", config.Outbounds)
	}
	stream := config.Outbounds[0].StreamSettings
	if stream.Network != "tcp" || stream.TCPSettings.Header.Type != "none" {
		t.Fatalf("stream settings = %#v, want standard TCP transport", stream)
	}
}

func TestBundledXrayAcceptsVMessRawURI(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("bundled Xray release is linux/amd64 only")
	}
	binaryPath, err := DefaultBinaryPath()
	if err != nil {
		t.Fatalf("DefaultBinaryPath(): %v", err)
	}
	profile := `{"ps":"raw-node","add":"server.example","port":9090,"id":"11111111-1111-4111-8111-111111111111","aid":0,"scy":"auto","net":"raw"}`
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(profile))
	generated, err := configFromURI(uri)
	if err != nil {
		t.Fatalf("configFromURI(): %v", err)
	}
	config, err := instrumentConfig(generated, "127.0.0.1:12345")
	if err != nil {
		t.Fatalf("instrumentConfig(): %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write generated config: %v", err)
	}
	output, err := exec.Command(binaryPath, "run", "-test", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("bundled Xray rejected VMess raw config: %v: %s", err, strings.TrimSpace(string(output)))
	}
}

func TestConfigFromURIWithOptions(t *testing.T) {
	uri := "vless://user@example.com:443"
	generated, err := configFromURIWithOptions(uri, "proxy", "127.0.0.1:9050")
	if err != nil {
		t.Fatalf("configFromURIWithOptions(proxy): %v", err)
	}
	var proxyConfig map[string]any
	if err := json.Unmarshal([]byte(generated), &proxyConfig); err != nil {
		t.Fatalf("decode proxy config: %v", err)
	}
	proxyInbound := proxyConfig["inbounds"].([]any)[0].(map[string]any)
	if proxyInbound["listen"] != "127.0.0.1" || proxyInbound["port"] != float64(9050) {
		t.Fatalf("proxy inbound = %#v", proxyInbound)
	}

	generated, err = configFromURIWithOptions(uri, "tunnel", "")
	if err != nil {
		t.Fatalf("configFromURIWithOptions(tunnel): %v", err)
	}
	var tunnelConfig map[string]any
	if err := json.Unmarshal([]byte(generated), &tunnelConfig); err != nil {
		t.Fatalf("decode tunnel config: %v", err)
	}
	tunnelInbound := tunnelConfig["inbounds"].([]any)[0].(map[string]any)
	if tunnelInbound["protocol"] != "tun" || tunnelInbound["tag"] != "local-tun" {
		t.Fatalf("tunnel inbound = %#v", tunnelInbound)
	}

	generated, err = configFromURIWithOptions(uri, "lan", "192.168.1.25:10808")
	if err != nil {
		t.Fatalf("configFromURIWithOptions(lan): %v", err)
	}
	var lanConfig map[string]any
	if err := json.Unmarshal([]byte(generated), &lanConfig); err != nil {
		t.Fatalf("decode LAN config: %v", err)
	}
	lanInbound := lanConfig["inbounds"].([]any)[0].(map[string]any)
	if lanInbound["listen"] != "192.168.1.25" || lanInbound["port"] != float64(10808) || lanInbound["tag"] != "lan-socks" {
		t.Fatalf("LAN inbound = %#v", lanInbound)
	}

	for _, address := range []string{"0.0.0.0:10808", "192.168.1.2:10808", "localhost", "127.0.0.1:0"} {
		if _, err := configFromURIWithOptions(uri, "proxy", address); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("proxy address %q error = %v, want ErrInvalidConfig", address, err)
		}
	}
	for _, address := range []string{"127.0.0.1:10808", "8.8.8.8:10808", "169.254.1.4:10808", "192.168.1.2:0"} {
		if _, err := configFromURIWithOptions(uri, "lan", address); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("LAN address %q error = %v, want ErrInvalidConfig", address, err)
		}
	}
}

func TestConfigFromURIRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{
		"",
		"https://example.com",
		"vless://user@example.com",
		"trojan://@example.com:443",
		"vmess://not-base64",
		"ss://method-password@example.com:8388",
	} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			if _, err := configFromURI(input); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("configFromURI(%q) error = %v, want ErrInvalidConfig", input, err)
			}
		})
	}
}

func TestBundledXrayAcceptsGeneratedURIConfig(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("bundled Xray release is linux/amd64 only")
	}
	binaryPath, err := DefaultBinaryPath()
	if err != nil {
		t.Fatalf("DefaultBinaryPath(): %v", err)
	}
	generated, err := configFromURI("vless://11111111-1111-4111-8111-111111111111@proxy.example:443?security=reality&sni=example.com&fp=chrome&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=1234&type=xhttp&host=cdn.example&path=%2Fedge")
	if err != nil {
		t.Fatalf("configFromURI(): %v", err)
	}
	config, err := instrumentConfig(generated, "127.0.0.1:12345")
	if err != nil {
		t.Fatalf("instrumentConfig(): %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write generated config: %v", err)
	}
	output, err := exec.Command(binaryPath, "run", "-test", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("bundled Xray rejected generated config: %v: %s", err, strings.TrimSpace(string(output)))
	}
}

func TestBundledXrayAcceptsGeneratedTunnelConfig(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("bundled Xray release is linux/amd64 only")
	}
	binaryPath, err := DefaultBinaryPath()
	if err != nil {
		t.Fatalf("DefaultBinaryPath(): %v", err)
	}
	generated, err := configFromURIWithOptions("vless://11111111-1111-4111-8111-111111111111@proxy.example:443", "tunnel", "")
	if err != nil {
		t.Fatalf("configFromURIWithOptions(): %v", err)
	}
	config, err := instrumentConfig(generated, "127.0.0.1:12345")
	if err != nil {
		t.Fatalf("instrumentConfig(): %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write generated config: %v", err)
	}
	output, err := exec.Command(binaryPath, "run", "-test", "-c", configPath).CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "operation not permitted") {
			t.Skip("TUN validation requires permission to create a network interface")
		}
		t.Fatalf("bundled Xray rejected generated tunnel config: %v: %s", err, strings.TrimSpace(string(output)))
	}
}
