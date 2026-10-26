package xray

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func configFromURI(rawURI string) (string, error) {
	return configFromURIWithOptions(rawURI, "proxy", "127.0.0.1:10808")
}

func configFromURIWithOptions(rawURI, mode, proxyAddress string) (string, error) {
	rawURI = strings.TrimSpace(rawURI)
	if rawURI == "" {
		return "", invalidURI("URI is empty")
	}
	var inbound map[string]any
	switch mode {
	case "proxy":
		host, port, err := loopbackAddress(proxyAddress)
		if err != nil {
			return "", err
		}
		inbound = map[string]any{
			"listen":   host,
			"port":     port,
			"protocol": "socks",
			"settings": map[string]any{"auth": "noauth", "udp": true},
			"tag":      "local-socks",
		}
	case "tunnel":
		inbound = map[string]any{
			"protocol": "tun",
			"settings": map[string]any{
				"name":       "packet-drift0",
				"MTU":        1500,
				"Address":    []string{"172.19.0.1/30"},
				"Gateway":    "172.19.0.2",
				"EnableIPv6": false,
			},
			"tag": "local-tun",
		}
	default:
		return "", invalidURI("connection mode must be proxy or tunnel")
	}

	var outbound map[string]any
	var tag string
	if strings.HasPrefix(strings.ToLower(rawURI), "vmess://") {
		var err error
		outbound, tag, err = parseVMessURI(rawURI)
		if err != nil {
			return "", err
		}
	} else {
		parsed, err := url.Parse(rawURI)
		if err != nil {
			return "", invalidURI("malformed URI")
		}
		tag = parsed.Fragment
		switch strings.ToLower(parsed.Scheme) {
		case "vless":
			outbound, err = parseVLESSURI(parsed)
		case "trojan":
			outbound, err = parseTrojanURI(parsed)
		case "ss":
			outbound, err = parseShadowsocksURI(parsed)
		default:
			return "", invalidURI("unsupported scheme; expected vless, vmess, trojan, or ss")
		}
		if err != nil {
			return "", err
		}
	}
	if tag == "" {
		tag = "proxy"
	}
	outbound["tag"] = tag
	config := map[string]any{
		"inbounds":  []any{inbound},
		"outbounds": []any{outbound},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("%w: encode generated config", ErrInvalidConfig)
	}
	return string(encoded), nil
}

func loopbackAddress(address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return "", 0, invalidURI("proxy address must be host:port")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", 0, invalidURI("proxy address must use a loopback host")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, invalidURI("proxy address must use a valid port")
	}
	return host, port, nil
}

func parseVLESSURI(uri *url.URL) (map[string]any, error) {
	if uri.User == nil || uri.User.Username() == "" || uri.User.String() != uri.User.Username() {
		return nil, invalidURI("vless URI must contain a UUID and must not contain a password")
	}
	address, port, err := uriAddress(uri)
	if err != nil {
		return nil, err
	}
	query := uri.Query()
	user := map[string]any{"id": uri.User.Username(), "encryption": query.Get("encryption")}
	if user["encryption"] == "" {
		user["encryption"] = "none"
	}
	if flow := query.Get("flow"); flow != "" {
		user["flow"] = flow
	}
	stream, err := streamSettings(query, "none")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"protocol": "vless",
		"settings": map[string]any{"vnext": []any{map[string]any{
			"address": address,
			"port":    port,
			"users":   []any{user},
		}}},
		"streamSettings": stream,
	}, nil
}

func parseTrojanURI(uri *url.URL) (map[string]any, error) {
	if uri.User == nil || uri.User.Username() == "" {
		return nil, invalidURI("trojan URI must contain a password")
	}
	password := uri.User.Username()
	if value, ok := uri.User.Password(); ok {
		password += ":" + value
	}
	address, port, err := uriAddress(uri)
	if err != nil {
		return nil, err
	}
	query := uri.Query()
	security := query.Get("security")
	if security == "" {
		security = "tls"
	}
	stream, err := streamSettings(query, security)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"protocol": "trojan",
		"settings": map[string]any{"servers": []any{map[string]any{
			"address":  address,
			"port":     port,
			"password": password,
		}}},
		"streamSettings": stream,
	}, nil
}

func parseShadowsocksURI(uri *url.URL) (map[string]any, error) {
	rawUserInfo := ""
	var address string
	var port int
	if uri.User != nil {
		var err error
		address, port, err = uriAddress(uri)
		if err != nil {
			return nil, err
		}
		rawUserInfo = uri.User.Username()
		if password, ok := uri.User.Password(); ok {
			rawUserInfo += ":" + password
		}
	} else {
		decoded, err := decodeBase64(strings.Trim(uri.Host, "[]"))
		if err != nil {
			return nil, invalidURI("invalid encoded Shadowsocks URI")
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return nil, invalidURI("Shadowsocks URI must contain method, password, and server")
		}
		rawUserInfo = decoded[:at]
		server, err := url.Parse("ss://" + decoded[at+1:])
		if err != nil {
			return nil, invalidURI("invalid Shadowsocks server address")
		}
		address, port, err = uriAddress(server)
		if err != nil {
			return nil, err
		}
	}
	if uri.Query().Get("plugin") != "" {
		return nil, invalidURI("Shadowsocks plugins are not supported")
	}
	separator := strings.IndexByte(rawUserInfo, ':')
	if separator <= 0 || separator == len(rawUserInfo)-1 {
		decoded, err := decodeBase64(rawUserInfo)
		if err == nil {
			rawUserInfo = decoded
			separator = strings.IndexByte(rawUserInfo, ':')
		}
	}
	if separator <= 0 || separator == len(rawUserInfo)-1 {
		return nil, invalidURI("Shadowsocks URI must contain a method and password")
	}
	return map[string]any{
		"protocol": "shadowsocks",
		"settings": map[string]any{"servers": []any{map[string]any{
			"address":  address,
			"port":     port,
			"method":   rawUserInfo[:separator],
			"password": rawUserInfo[separator+1:],
		}}},
	}, nil
}

func parseVMessURI(rawURI string) (map[string]any, string, error) {
	payload := strings.TrimSpace(rawURI[len("vmess://"):])
	if fragment := strings.IndexByte(payload, '#'); fragment >= 0 {
		payload = payload[:fragment]
	}
	decoded, err := decodeBase64(payload)
	if err != nil {
		return nil, "", invalidURI("invalid encoded VMess URI")
	}
	var profile struct {
		Name        string          `json:"ps"`
		Address     string          `json:"add"`
		Port        json.RawMessage `json:"port"`
		ID          string          `json:"id"`
		AlterID     json.RawMessage `json:"aid"`
		Cipher      string          `json:"scy"`
		Network     string          `json:"net"`
		HeaderType  string          `json:"type"`
		Host        string          `json:"host"`
		Path        string          `json:"path"`
		TLS         string          `json:"tls"`
		ServerName  string          `json:"sni"`
		Fingerprint string          `json:"fp"`
		ALPN        string          `json:"alpn"`
	}
	if err := json.Unmarshal([]byte(decoded), &profile); err != nil {
		return nil, "", invalidURI("invalid VMess profile")
	}
	port, err := scalarInt(profile.Port)
	if err != nil || profile.Address == "" || profile.ID == "" {
		return nil, "", invalidURI("VMess URI must contain address, port, and UUID")
	}
	alterID := 0
	if len(profile.AlterID) > 0 {
		alterID, _ = scalarInt(profile.AlterID)
	}
	cipher := profile.Cipher
	if cipher == "" {
		cipher = "auto"
	}
	user := map[string]any{"id": profile.ID, "alterId": alterID, "security": cipher}
	query := make(url.Values)
	query.Set("type", profile.Network)
	query.Set("headerType", profile.HeaderType)
	query.Set("host", profile.Host)
	query.Set("path", profile.Path)
	query.Set("security", profile.TLS)
	query.Set("sni", profile.ServerName)
	query.Set("fp", profile.Fingerprint)
	query.Set("alpn", profile.ALPN)
	security := profile.TLS
	if security == "" || security == "tls" {
		security = "tls"
	}
	stream, err := streamSettings(query, security)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{
		"protocol": "vmess",
		"settings": map[string]any{"vnext": []any{map[string]any{
			"address": profile.Address,
			"port":    port,
			"users":   []any{user},
		}}},
		"streamSettings": stream,
	}, profile.Name, nil
}

func streamSettings(query url.Values, defaultSecurity string) (map[string]any, error) {
	network := query.Get("type")
	if network == "" {
		network = "tcp"
	}
	stream := map[string]any{"network": network}
	security := query.Get("security")
	if security == "" {
		security = defaultSecurity
	}
	switch security {
	case "", "none":
		security = "none"
	case "tls":
		tlsSettings := map[string]any{}
		if serverName := firstNonEmpty(query.Get("sni"), query.Get("serverName")); serverName != "" {
			tlsSettings["serverName"] = serverName
		}
		if fingerprint := query.Get("fp"); fingerprint != "" {
			tlsSettings["fingerprint"] = fingerprint
		}
		if alpn := query.Get("alpn"); alpn != "" {
			tlsSettings["alpn"] = strings.Split(alpn, ",")
		}
		stream["tlsSettings"] = tlsSettings
	case "reality":
		realitySettings := map[string]any{}
		if serverName := firstNonEmpty(query.Get("sni"), query.Get("serverName")); serverName != "" {
			realitySettings["serverName"] = serverName
		}
		if fingerprint := query.Get("fp"); fingerprint != "" {
			realitySettings["fingerprint"] = fingerprint
		}
		if key := query.Get("pbk"); key != "" {
			realitySettings["publicKey"] = key
		}
		if shortID := query.Get("sid"); shortID != "" {
			realitySettings["shortId"] = shortID
		}
		if spiderX := query.Get("spx"); spiderX != "" {
			realitySettings["spiderX"] = spiderX
		}
		stream["realitySettings"] = realitySettings
	default:
		return nil, invalidURI("unsupported transport security: " + security)
	}
	stream["security"] = security
	if security == "reality" && network != "tcp" && network != "grpc" && network != "xhttp" {
		return nil, invalidURI("Reality supports tcp, grpc, and xhttp transports")
	}

	switch network {
	case "tcp":
		headerType := query.Get("headerType")
		if headerType == "http" {
			headers := map[string]any{}
			if host := query.Get("host"); host != "" {
				headers["Host"] = strings.Split(host, ",")
			}
			request := map[string]any{"path": []string{firstNonEmpty(query.Get("path"), "/")}, "headers": headers}
			stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "http", "request": request}}
		} else {
			stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
		}
	case "ws":
		headers := map[string]any{}
		if host := query.Get("host"); host != "" {
			headers["Host"] = host
		}
		stream["wsSettings"] = map[string]any{"path": firstNonEmpty(query.Get("path"), "/"), "headers": headers}
	case "grpc":
		grpc := map[string]any{"serviceName": query.Get("serviceName")}
		if authority := query.Get("authority"); authority != "" {
			grpc["authority"] = authority
		}
		stream["grpcSettings"] = grpc
	case "xhttp":
		xhttp := map[string]any{"path": firstNonEmpty(query.Get("path"), "/")}
		if host := query.Get("host"); host != "" {
			xhttp["host"] = host
		}
		if mode := query.Get("mode"); mode != "" {
			xhttp["mode"] = mode
		}
		stream["xhttpSettings"] = xhttp
	case "http", "h2":
		stream["httpSettings"] = map[string]any{"path": firstNonEmpty(query.Get("path"), "/"), "host": strings.Split(query.Get("host"), ",")}
	default:
		return nil, invalidURI("unsupported transport type: " + network)
	}
	return stream, nil
}

func uriAddress(uri *url.URL) (string, int, error) {
	address := uri.Hostname()
	port, err := strconv.Atoi(uri.Port())
	if address == "" || err != nil || port < 1 || port > 65535 {
		return "", 0, invalidURI("server address and valid port are required")
	}
	return address, port, nil
}

func decodeBase64(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("invalid base64")
}

func scalarInt(value json.RawMessage) (int, error) {
	var number json.Number
	if err := json.Unmarshal(value, &number); err == nil {
		return strconv.Atoi(number.String())
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return 0, err
	}
	return strconv.Atoi(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func invalidURI(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidConfig, reason)
}
