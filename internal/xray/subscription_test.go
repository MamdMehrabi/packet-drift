package xray

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSubscriptionContent(t *testing.T) {
	content := "vless://user@first.example:443#First\nss://aes-128-gcm:secret@second.example:8388#Second\nvless://user@first.example:443#First"
	nodes, err := parseSubscriptionContent(content)
	if err != nil {
		t.Fatalf("parseSubscriptionContent(): %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Name != "First" || nodes[1].Name != "Second" {
		t.Fatalf("node names = %q, %q", nodes[0].Name, nodes[1].Name)
	}
}

func TestParseBase64SubscriptionContent(t *testing.T) {
	content := "trojan://secret@server.example:443#Tokyo"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	encoded = encoded[:len(encoded)/2] + "\n" + encoded[len(encoded)/2:]
	nodes, err := parseSubscriptionContent(encoded)
	if err != nil {
		t.Fatalf("parseSubscriptionContent(): %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Tokyo" {
		t.Fatalf("nodes = %#v", nodes)
	}
}

func TestParseSubscriptionContentRejectsInvalidProxyURI(t *testing.T) {
	if _, err := parseSubscriptionContent("vless://invalid"); err == nil {
		t.Fatal("parseSubscriptionContent() accepted an invalid proxy URI")
	}
}

func TestFetchSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("request method = %q, want GET", request.Method)
		}
		_, _ = writer.Write([]byte("vless://user@node.example:443#Node"))
	}))
	defer server.Close()

	nodes, err := FetchSubscription(server.URL + "/subscription")
	if err != nil {
		t.Fatalf("FetchSubscription(): %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Node" {
		t.Fatalf("nodes = %#v", nodes)
	}
}

func TestFetchSubscriptionRejectsUnsupportedURLs(t *testing.T) {
	for _, rawURL := range []string{"file:///tmp/sub", "ftp://example.com/sub", "https://user:secret@example.com/sub"} {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := FetchSubscription(rawURL); err == nil {
				t.Fatal("FetchSubscription() accepted an unsupported URL")
			}
		})
	}
}

func TestFetchSubscriptionRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxSubscriptionBytes+1)))
	}))
	defer server.Close()
	if _, err := FetchSubscription(server.URL); err == nil || !strings.Contains(err.Error(), "2 MiB") {
		t.Fatalf("FetchSubscription() error = %v, want size limit error", err)
	}
}