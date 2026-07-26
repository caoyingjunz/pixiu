/*
Copyright 2026 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/remotedialer"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/tunnel"
)

func main() {
	server := strings.TrimSpace(os.Getenv("PIXIU_SERVER"))
	token := strings.TrimSpace(os.Getenv("PIXIU_TOKEN"))
	if server == "" || token == "" {
		klog.Fatalf("PIXIU_SERVER and PIXIU_TOKEN are required")
	}

	wsURL, err := buildConnectURL(server)
	if err != nil {
		klog.Fatalf("invalid PIXIU_SERVER: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.TODO(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	headers := http.Header{}
	headers.Set(tunnel.TokenHeader, token)

	insecure := strings.EqualFold(os.Getenv("PIXIU_INSECURE"), "true")
	dialer := websocket.DefaultDialer
	if insecure {
		dialer = &websocket.Dialer{
			Proxy:            http.ProxyFromEnvironment,
			HandshakeTimeout: 45 * time.Second,
			TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	klog.Infof("cluster-agent connecting to %s", wsURL)
	for {
		if ctx.Err() != nil {
			return
		}
		err := remotedialer.ClientConnect(ctx, wsURL, headers, dialer, func(proto, address string) bool {
			return proto == "tcp"
		}, func(ctx context.Context, _ *remotedialer.Session) error {
			klog.Infof("tunnel connected, waiting for dial requests")
			<-ctx.Done()
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		klog.Errorf("tunnel disconnected: %v; retrying in 5s", err)
		time.Sleep(5 * time.Second)
	}
}

func buildConnectURL(server string) (string, error) {
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// ok
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = tunnel.ConnectPath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
