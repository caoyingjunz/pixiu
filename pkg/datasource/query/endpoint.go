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

package query

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// Endpoint 描述一次上游查询所需的连接信息（与前端实时查询语义对齐）。
type Endpoint struct {
	SubType     model.DatasourceSubType
	External    bool
	ClusterName string
	BaseURL     string
	Username    string
	Password    string
	Headers     map[string]string
}

// InClusterEndpoint 解析后的集群内 Service 地址。
type InClusterEndpoint struct {
	ServiceName string
	Namespace   string
	Port        int
	Scheme      string // http / https
	BasePath    string
}

// ParseEndpoint 从数据库 Datasource 解析查询端点。
func ParseEndpoint(ds *model.Datasource) (*Endpoint, error) {
	if ds == nil {
		return nil, fmt.Errorf("datasource is nil")
	}
	var cfg types.DatasourceConfig
	if err := cfg.Unmarshal(ds.Config); err != nil {
		return nil, fmt.Errorf("invalid datasource config: %w", err)
	}

	ep := &Endpoint{
		SubType:     ds.SubType,
		External:    ds.External,
		ClusterName: strings.TrimSpace(ds.ClusterName),
		Headers:     headerMap(cfg.Headers),
	}

	switch ds.Type {
	case model.DatasourceTypeAlert:
		if cfg.Alert == nil || strings.TrimSpace(cfg.Alert.URL) == "" {
			return nil, fmt.Errorf("datasource %d missing alert.url", ds.Id)
		}
		ep.BaseURL = strings.TrimSpace(cfg.Alert.URL)
		ep.Username = cfg.Alert.UserName
		ep.Password = cfg.Alert.Password
	case model.DatasourceTypeLog:
		if cfg.Log == nil || strings.TrimSpace(cfg.Log.URL) == "" {
			return nil, fmt.Errorf("datasource %d missing log.url", ds.Id)
		}
		ep.BaseURL = strings.TrimSpace(cfg.Log.URL)
		ep.Username = cfg.Log.UserName
		ep.Password = cfg.Log.Password
	default:
		return nil, fmt.Errorf("unsupported datasource type %d", ds.Type)
	}

	ep.BaseURL = strings.TrimRight(ep.BaseURL, "/")
	if !ep.External && ep.ClusterName == "" {
		return nil, fmt.Errorf("internal datasource %d missing cluster_name", ds.Id)
	}
	return ep, nil
}

func headerMap(headers []types.HTTPHeader) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		key := strings.TrimSpace(h.Key)
		value := strings.TrimSpace(h.Value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ParseInClusterEndpoint mirrors pixiu-ui parseInClusterDatasourceEndpoint.
func ParseInClusterEndpoint(rawURL string) (*InClusterEndpoint, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty datasource url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid datasource url: %w", err)
	}
	host := parsed.Hostname()
	if host == "" || isIPAddress(host) {
		return nil, fmt.Errorf("internal datasource url must be an in-cluster DNS name")
	}

	scheme := strings.ToLower(strings.TrimSuffix(parsed.Scheme, ":"))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}

	parts := strings.Split(host, ".")
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("invalid hostname %q", host)
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
		return nil, fmt.Errorf("cannot resolve namespace from hostname %q", host)
	}

	port := 0
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil || port <= 0 {
			return nil, fmt.Errorf("invalid port in datasource url")
		}
	} else if scheme == "https" {
		port = 443
	} else {
		port = 80
	}

	return &InClusterEndpoint{
		ServiceName: filtered[0],
		Namespace:   namespace,
		Port:        port,
		Scheme:      scheme,
		BasePath:    normalizeBasePath(parsed.Path),
	}, nil
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
	// crude IPv6 without brackets
	if strings.Contains(hostname, ":") && !strings.Contains(hostname, ".") {
		return true
	}
	return false
}

func joinAPIPath(basePath, apiPath string) string {
	apiPath = strings.TrimSpace(apiPath)
	if apiPath == "" {
		return basePath
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	return basePath + apiPath
}
