package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSubscriptionBytes = 2 << 20

type SubscriptionNode struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

func FetchSubscription(rawURL string) ([]SubscriptionNode, error) {
	parsedURL, err := validSubscriptionURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("subscription: too many redirects")
			}
			if _, err := validSubscriptionURL(request.URL.String()); err != nil {
				return err
			}
			if via[0].URL.Scheme == "https" && request.URL.Scheme != "https" {
				return fmt.Errorf("subscription: refusing HTTPS downgrade redirect")
			}
			return nil
		},
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("subscription: invalid URL")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("subscription: fetch failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subscription: server returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSubscriptionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("subscription: read response: %w", err)
	}
	if len(body) > maxSubscriptionBytes {
		return nil, fmt.Errorf("subscription: response exceeds 2 MiB limit")
	}
	return parseSubscriptionContent(string(body))
}

func validSubscriptionURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return nil, fmt.Errorf("subscription: enter a valid HTTP or HTTPS URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("subscription: only HTTP and HTTPS URLs are supported")
	}
	return parsed, nil
}

func parseSubscriptionContent(content string) ([]SubscriptionNode, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("subscription: response is empty")
	}
	if !containsSupportedURI(content) {
		compact := strings.Map(func(char rune) rune {
			if char == ' ' || char == '\t' || char == '\r' || char == '\n' {
				return -1
			}
			return char
		}, content)
		decoded, err := decodeBase64(compact)
		if err != nil || !containsSupportedURI(decoded) {
			return nil, fmt.Errorf("subscription: response contains no supported proxy links")
		}
		content = decoded
	}

	nodes := make([]SubscriptionNode, 0)
	seen := make(map[string]struct{})
	for lineNumber, line := range strings.Split(content, "\n") {
		uri := strings.TrimSpace(line)
		if !isSupportedURI(uri) {
			continue
		}
		if _, exists := seen[uri]; exists {
			continue
		}
		generated, err := configFromURI(uri)
		if err != nil {
			return nil, fmt.Errorf("subscription: invalid proxy link on line %d: %w", lineNumber+1, err)
		}
		var config struct {
			Outbounds []struct {
				Tag string `json:"tag"`
			} `json:"outbounds"`
		}
		if err := json.Unmarshal([]byte(generated), &config); err != nil || len(config.Outbounds) == 0 {
			return nil, fmt.Errorf("subscription: could not read proxy link on line %d", lineNumber+1)
		}
		name := strings.TrimSpace(config.Outbounds[0].Tag)
		if name == "" || name == "proxy" {
			name = fmt.Sprintf("Node %d", len(nodes)+1)
		}
		nodes = append(nodes, SubscriptionNode{Name: name, URI: uri})
		seen[uri] = struct{}{}
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("subscription: response contains no supported proxy links")
	}
	return nodes, nil
}

func containsSupportedURI(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if isSupportedURI(strings.TrimSpace(line)) {
			return true
		}
	}
	return false
}

func isSupportedURI(uri string) bool {
	lower := strings.ToLower(uri)
	return strings.HasPrefix(lower, "vless://") || strings.HasPrefix(lower, "vmess://") ||
		strings.HasPrefix(lower, "trojan://") || strings.HasPrefix(lower, "ss://")
}