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

package datasource

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// InClusterEndpoint mirrors frontend parseInClusterDatasourceEndpoint:
// e.g. http://prometheus-server.pixiu-system -> service/namespace/port.
type InClusterEndpoint struct {
	ServiceName string
	Namespace   string
	Port        int
	Protocol    string // http | https
	BasePath    string // optional path prefix, e.g. /prometheus
}

// ParseInClusterEndpoint parses a cluster-local datasource URL.
// Returns nil when the URL is not a valid in-cluster HTTP(S) service address.
func ParseInClusterEndpoint(rawURL string) (*InClusterEndpoint, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty datasource url")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid datasource url: %w", err)
	}

	protocol := strings.ToLower(parsed.Scheme)
	switch protocol {
	case "http", "https":
	default:
		return nil, fmt.Errorf("datasource url must use http or https")
	}

	host := parsed.Hostname()
	if host == "" || isIPAddress(host) {
		return nil, fmt.Errorf("datasource url host must be a cluster DNS name")
	}

	parts := strings.Split(host, ".")
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("datasource url host is empty")
	}

	svcIndex := -1
	for i, p := range filtered {
		if p == "svc" {
			svcIndex = i
			break
		}
	}

	var namespace string
	switch {
	case svcIndex >= 2:
		namespace = filtered[1]
	case svcIndex == -1 && len(filtered) >= 2:
		namespace = filtered[1]
	default:
		return nil, fmt.Errorf("cannot resolve namespace from datasource url host %q", host)
	}
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace in datasource url host %q", host)
	}

	port := 0
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil || port <= 0 {
			return nil, fmt.Errorf("invalid port in datasource url")
		}
	} else if protocol == "https" {
		port = 443
	} else {
		port = 80
	}

	return &InClusterEndpoint{
		ServiceName: filtered[0],
		Namespace:   namespace,
		Port:        port,
		Protocol:    protocol,
		BasePath:    normalizeBasePath(parsed.Path),
	}, nil
}

// JoinProxyPath builds the path passed to Kubernetes Service ProxyGet.
func (e *InClusterEndpoint) JoinProxyPath(apiPath string) string {
	apiPath = strings.TrimSpace(apiPath)
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	base := e.BasePath
	if base == "" {
		return strings.TrimPrefix(apiPath, "/")
	}
	return strings.TrimPrefix(base+apiPath, "/")
}

func normalizeBasePath(pathname string) string {
	normalized := strings.TrimSpace(pathname)
	normalized = strings.TrimRight(normalized, "/")
	if normalized == "" || normalized == "/" {
		return ""
	}
	if !strings.HasPrefix(normalized, "/") {
		return "/" + normalized
	}
	return normalized
}

func isIPAddress(hostname string) bool {
	if net.ParseIP(hostname) != nil {
		return true
	}
	// IPv6 without brackets rarely appears in Hostname(); keep frontend-compatible heuristic.
	if strings.Contains(hostname, ":") && !strings.Contains(hostname, ".") {
		return true
	}
	return false
}
