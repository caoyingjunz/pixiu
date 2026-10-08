/*
Copyright 2021 The Pixiu Authors.

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

package proxy

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const upstreamDatasourceIDHeader = "X-Pixiu-Datasource-Id"

// resolveProxyDatasource consumes the datasource ID before forwarding upstream.
func (p *proxyRouter) resolveProxyDatasource(c *gin.Context) (*types.Datasource, error) {
	dsIDStr := strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader))
	if dsIDStr == "" {
		return nil, nil
	}
	c.Request.Header.Del(upstreamDatasourceIDHeader)

	datasourceID, err := strconv.ParseInt(dsIDStr, 10, 64)
	if err != nil || datasourceID <= 0 {
		return nil, fmt.Errorf("invalid datasource id")
	}
	datasource, err := p.c.Datasource().GetForProxy(c, datasourceID)
	if err != nil {
		return nil, err
	}
	if datasource == nil {
		return nil, fmt.Errorf("datasource %d not found", datasourceID)
	}
	return datasource, nil
}

// datasourceBasicAuthorization returns HTTP Basic credentials for non-Nacos datasources.
func datasourceBasicAuthorization(datasource *types.Datasource) string {
	if datasource == nil || datasource.SubType == model.DatasourceSubTypeNacos {
		return ""
	}

	var username, password string
	switch datasource.Type {
	case model.DatasourceTypeLog, model.DatasourceTypeMiddleware:
		// 中间件（如 Nacos）的鉴权账号复用 log 配置存储
		if datasource.Config.Log == nil {
			return ""
		}
		username = datasource.Config.Log.UserName
		password = datasource.Config.Log.Password
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert == nil {
			return ""
		}
		username = datasource.Config.Alert.UserName
		password = datasource.Config.Alert.Password
	default:
		return ""
	}

	if strings.TrimSpace(username) == "" && strings.TrimSpace(password) == "" {
		return ""
	}

	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}

func datasourceBaseURL(datasource *types.Datasource) (string, error) {
	if datasource == nil {
		return "", fmt.Errorf("datasource is required")
	}
	switch datasource.Type {
	case model.DatasourceTypeLog, model.DatasourceTypeMiddleware:
		if datasource.Config.Log != nil && strings.TrimSpace(datasource.Config.Log.URL) != "" {
			return datasource.Config.Log.URL, nil
		}
	case model.DatasourceTypeAlert:
		if datasource.Config.Alert != nil && strings.TrimSpace(datasource.Config.Alert.URL) != "" {
			return datasource.Config.Alert.URL, nil
		}
	}
	return "", fmt.Errorf("datasource %d has no proxy URL", datasource.Id)
}

// validateExternalDatasourceTarget binds datasource credentials to their configured origin.
func validateExternalDatasourceTarget(datasource *types.Datasource, target *url.URL) error {
	baseURL, err := datasourceBaseURL(datasource)
	if err != nil {
		return err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid datasource URL: %w", err)
	}
	if target == nil || !strings.EqualFold(base.Hostname(), target.Hostname()) || base.Scheme != target.Scheme {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	// Nacos 3.x may expose its Console API on 8080 while the configured client
	// endpoint uses another port. The hostname still must match.
	if datasource.SubType != model.DatasourceSubTypeNacos && effectivePort(base) != effectivePort(target) {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	return nil
}

// validateInternalDatasourceTarget binds datasource credentials to their Service target.
func validateInternalDatasourceTarget(datasource *types.Datasource, target *serviceProxyTarget) error {
	baseURL, err := datasourceBaseURL(datasource)
	if err != nil {
		return err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid datasource URL: %w", err)
	}
	parts := strings.Split(base.Hostname(), ".")
	if len(parts) < 2 || target == nil || parts[0] != target.service || parts[1] != target.namespace {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	if effectivePort(base) != target.port {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	return nil
}

func effectivePort(raw *url.URL) int {
	if raw == nil {
		return 0
	}
	if port, err := strconv.Atoi(raw.Port()); err == nil && port > 0 {
		return port
	}
	if raw.Scheme == "https" {
		return 443
	}
	return 80
}
