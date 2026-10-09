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
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/pkg/controller/datasource"
	datasourceauth "github.com/caoyingjunz/pixiu/pkg/datasource/auth"
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
	datasource, err := p.c.Datasource().Get(c, datasourceID, datasource.WithCredentials())
	if err != nil {
		return nil, err
	}
	if datasource == nil {
		return nil, fmt.Errorf("datasource %d not found", datasourceID)
	}
	return datasource, nil
}

// validateInternalDatasourceTarget binds datasource credentials to their Service target.
func validateInternalDatasourceTarget(datasource *types.Datasource, target *serviceProxyTarget) error {
	baseURL, err := datasourceauth.DatasourceURL(datasource)
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
	if datasourceauth.EffectivePort(base) != target.port {
		return fmt.Errorf("proxy target does not match datasource %d", datasource.Id)
	}
	return nil
}
