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
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const upstreamDatasourceIDHeader = "X-Pixiu-Datasource-Id"

func (p *proxyRouter) resolveUpstreamAuthFromDatasource(c *gin.Context) string {
	rawID := strings.TrimSpace(c.Request.Header.Get(upstreamDatasourceIDHeader))
	c.Request.Header.Del(upstreamDatasourceIDHeader)
	if rawID == "" {
		return ""
	}

	datasourceID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || datasourceID <= 0 {
		return ""
	}

	datasource, err := p.c.Datasource().Get(context.TODO(), datasourceID)
	if err != nil || datasource == nil || datasource.Config.Log == nil {
		return ""
	}

	username := datasource.Config.Log.UserName
	password := datasource.Config.Log.Password
	if username == "" && password == "" {
		return ""
	}

	token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + token
}
