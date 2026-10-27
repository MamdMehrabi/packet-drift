package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

const serverPingTimeout = 4 * time.Second

func CheckServerLatency(rawURI string) (int64, error) {
	configJSON, err := configFromURI(rawURI)
	if err != nil {
		return 0, err
	}
	var config struct {
		Outbounds []struct {
			Settings struct {
				VNext []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
				} `json:"vnext"`
				Servers []struct {
					Address string `json:"address"`
					Port    int    `json:"port"`
				} `json:"servers"`
			} `json:"settings"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil || len(config.Outbounds) != 1 {
		return 0, invalidURI("could not read server address from proxy URI")
	}
	settings := config.Outbounds[0].Settings
	var address string
	var port int
	if len(settings.VNext) > 0 {
		address, port = settings.VNext[0].Address, settings.VNext[0].Port
	} else if len(settings.Servers) > 0 {
		address, port = settings.Servers[0].Address, settings.Servers[0].Port
	}
	if address == "" || port < 1 || port > 65535 {
		return 0, invalidURI("could not read server address from proxy URI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverPingTimeout)
	defer cancel()
	startedAt := time.Now()
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		return 0, fmt.Errorf("server is unreachable: %w", err)
	}
	_ = connection.Close()
	latency := time.Since(startedAt)
	return int64((latency + time.Millisecond - 1) / time.Millisecond), nil
}
