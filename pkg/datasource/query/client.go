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
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const defaultHTTPTimeout = 15 * time.Second

// Request 描述对上游数据源的一次 HTTP 请求（apiPath 不含数据源 BaseURL）。
type Request struct {
	Method  string
	APIPath string
	Query   url.Values
	Headers map[string]string
	Body    io.Reader
}

// Client 按数据源类型 / 内外网语义发送请求（与实时查询一致）。
type Client struct {
	factory    db.ShareDaoFactory
	httpClient *http.Client
}

func NewClient(factory db.ShareDaoFactory) *Client {
	return &Client{
		factory: factory,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do 根据 datasource.external / cluster_name / sub_type 路由请求。
func (c *Client) Do(ctx context.Context, ds *model.Datasource, req Request) ([]byte, int, error) {
	if ds == nil {
		return nil, 0, fmt.Errorf("datasource is nil")
	}
	ep, err := ParseEndpoint(ds)
	if err != nil {
		return nil, 0, err
	}
	return c.doEndpoint(ctx, ep, req)
}

func (c *Client) doEndpoint(ctx context.Context, ep *Endpoint, req Request) ([]byte, int, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	if ep.External {
		return c.doExternal(ctx, ep, method, req)
	}
	return c.doInternal(ctx, ep, method, req)
}

func (c *Client) doExternal(ctx context.Context, ep *Endpoint, method string, req Request) ([]byte, int, error) {
	target, err := url.Parse(ep.BaseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid datasource url: %w", err)
	}
	apiPath := strings.TrimSpace(req.APIPath)
	if apiPath == "" {
		apiPath = "/"
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	target.Path = strings.TrimRight(target.Path, "/") + apiPath
	if len(req.Query) > 0 {
		target.RawQuery = req.Query.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, target.String(), req.Body)
	if err != nil {
		return nil, 0, err
	}
	applyAuthAndHeaders(httpReq, ep, req.Headers)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (c *Client) doInternal(ctx context.Context, ep *Endpoint, method string, req Request) ([]byte, int, error) {
	inCluster, err := ParseInClusterEndpoint(ep.BaseURL)
	if err != nil {
		return nil, 0, err
	}
	if ep.ClusterName == "" {
		return nil, 0, fmt.Errorf("internal datasource missing cluster_name")
	}

	cluster, err := c.factory.Cluster().GetClusterByName(ctx, ep.ClusterName)
	if err != nil {
		return nil, 0, err
	}
	if cluster == nil {
		return nil, 0, fmt.Errorf("cluster %q not found", ep.ClusterName)
	}
	clusterSet, err := client.NewClusterSet(cluster.KubeConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build cluster client: %w", err)
	}

	proxyPath := joinAPIPath(inCluster.BasePath, req.APIPath)
	proxyPath = strings.TrimPrefix(proxyPath, "/")

	params := map[string]string{}
	for key, values := range req.Query {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}

	// Prefer Kubernetes service proxy (same path family as /pixiu/proxy/.../services/.../proxy).
	raw, err := clusterSet.Client.CoreV1().Services(inCluster.Namespace).ProxyGet(
		inCluster.Scheme,
		inCluster.ServiceName,
		strconv.Itoa(inCluster.Port),
		proxyPath,
		params,
	).DoRaw(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"cluster service proxy %s/%s:%d failed: %w",
			inCluster.Namespace, inCluster.ServiceName, inCluster.Port, err,
		)
	}
	_ = method // ProxyGet is GET-oriented; metric query only needs GET today.
	return raw, http.StatusOK, nil
}

func applyAuthAndHeaders(req *http.Request, ep *Endpoint, extra map[string]string) {
	for key, value := range ep.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	for key, value := range extra {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	if ep.Username != "" || ep.Password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(ep.Username + ":" + ep.Password))
		req.Header.Set("Authorization", "Basic "+token)
	}
}

// ResolveAlertDatasource 按规则绑定 / 默认 prometheus 告警源解析数据源。
func ResolveAlertDatasource(ctx context.Context, factory db.ShareDaoFactory, datasourceID int64) (*model.Datasource, error) {
	if datasourceID > 0 {
		ds, err := factory.Datasource().Get(ctx, datasourceID)
		if err != nil {
			return nil, err
		}
		if ds == nil {
			return nil, fmt.Errorf("datasource %d not found", datasourceID)
		}
		if ds.Type != model.DatasourceTypeAlert {
			return nil, fmt.Errorf("datasource %d is not an alert datasource", datasourceID)
		}
		return ds, nil
	}

	list, err := factory.Datasource().List(ctx,
		db.WithDatasourceType(model.DatasourceTypeAlert),
		db.WithDatasourceSubType(model.DatasourceSubTypePrometheus),
		db.WithDatasourceIsDefault(true),
	)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		list, err = factory.Datasource().List(ctx,
			db.WithDatasourceType(model.DatasourceTypeAlert),
			db.WithDatasourceSubType(model.DatasourceSubTypePrometheus),
		)
		if err != nil {
			return nil, err
		}
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}
