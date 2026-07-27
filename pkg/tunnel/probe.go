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

package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultProbeTimeout bounds a single control-plane tunnel dial probe.
	DefaultProbeTimeout = 5 * time.Second
)

// AgentConnected returns the last control-plane probe result when available,
// otherwise falls back to whether a remotedialer session currently exists.
func (m *Manager) AgentConnected(clusterName string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	if m.connected != nil {
		if v, ok := m.connected[clusterName]; ok {
			m.mu.RUnlock()
			return v
		}
	}
	m.mu.RUnlock()
	return m.HasSession(clusterName)
}

// SetAgentConnected records the latest control-plane connectivity check result.
func (m *Manager) SetAgentConnected(clusterName string, connected bool) {
	if m == nil || clusterName == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connected == nil {
		m.connected = make(map[string]bool)
	}
	m.connected[clusterName] = connected
}

// ClearAgentConnected drops cached probe state for a cluster.
func (m *Manager) ClearAgentConnected(clusterName string) {
	if m == nil || clusterName == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connected, clusterName)
}

// Probe reports whether the agent tunnel is usable for the given cluster.
// It requires an active session and a successful TCP dial to apiServerURL
// through the tunnel (kube-apiserver host from kubeconfig).
func (m *Manager) Probe(ctx context.Context, clusterName, apiServerURL string) bool {
	if m == nil || !m.HasSession(clusterName) {
		return false
	}
	addr, err := dialAddressFromAPIServer(apiServerURL)
	if err != nil || addr == "" {
		// Session exists but we cannot derive a probe target; treat as connected
		// at the transport layer only.
		return true
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultProbeTimeout)
		defer cancel()
	}

	conn, err := m.Dialer(clusterName)(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func dialAddressFromAPIServer(apiServerURL string) (string, error) {
	host := strings.TrimSpace(apiServerURL)
	if host == "" {
		return "", fmt.Errorf("empty api server url")
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("missing host in %q", apiServerURL)
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		default:
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}
