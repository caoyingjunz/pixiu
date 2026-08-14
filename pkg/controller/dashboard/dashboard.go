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

package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	datasourcequery "github.com/caoyingjunz/pixiu/pkg/datasource/query"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	defaultStep   = 60 * time.Second
	maximumRange  = 7 * 24 * time.Hour
	queryParallel = 6
)

type Getter interface {
	Dashboard() Interface
}

type Interface interface {
	Definition() Definition
	Variables(ctx context.Context, req VariableRequest) (*Variables, error)
	Query(ctx context.Context, req QueryRequest) (*QueryResponse, error)
}

type controller struct {
	factory db.ShareDaoFactory
	client  *datasourcequery.Client
}

func New(factory db.ShareDaoFactory) Interface {
	return &controller{factory: factory, client: datasourcequery.NewClient(factory)}
}

func (c *controller) Definition() Definition { return Catalog() }

func (c *controller) loadDatasource(ctx context.Context, id int64) (*model.Datasource, error) {
	if id <= 0 {
		return nil, apierrors.NewError(fmt.Errorf("datasource_id is required"), http.StatusBadRequest)
	}
	ds, err := c.factory.Datasource().Get(ctx, id)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if ds == nil {
		return nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if ds.Type != model.DatasourceTypeAlert || ds.SubType != model.DatasourceSubTypePrometheus {
		return nil, apierrors.NewError(fmt.Errorf("a Prometheus alert datasource is required"), http.StatusBadRequest)
	}
	if err := controllerutil.CheckResourceAccess(ctx, c.factory, ds.UserId, types.ResourceTypeDatasource, ds.Id); err != nil {
		return nil, err
	}
	return ds, nil
}

func (c *controller) Variables(ctx context.Context, req VariableRequest) (*Variables, error) {
	ds, err := c.loadDatasource(ctx, req.DatasourceID)
	if err != nil {
		return nil, err
	}
	result := &Variables{Namespaces: []string{}, Nodes: []string{}, Workloads: []WorkloadOption{}, Pods: []string{}}

	queries := []struct {
		name       string
		expression string
		consume    func([]datasourcequery.Series)
	}{
		{name: "namespaces", expression: "kube_namespace_created", consume: func(series []datasourcequery.Series) { result.Namespaces = uniqueLabels(series, "namespace") }},
		{name: "nodes", expression: "kube_node_info", consume: func(series []datasourcequery.Series) { result.Nodes = uniqueLabels(series, "node") }},
		{name: "deployments", expression: variableSelector("kube_deployment_status_replicas", "namespace", req.Namespace), consume: func(series []datasourcequery.Series) { appendWorkloads(result, series, "deployment", "Deployment") }},
		{name: "statefulsets", expression: variableSelector("kube_statefulset_replicas", "namespace", req.Namespace), consume: func(series []datasourcequery.Series) { appendWorkloads(result, series, "statefulset", "StatefulSet") }},
		{name: "daemonsets", expression: variableSelector("kube_daemonset_status_desired_number_scheduled", "namespace", req.Namespace), consume: func(series []datasourcequery.Series) { appendWorkloads(result, series, "daemonset", "DaemonSet") }},
		{name: "jobs", expression: variableSelector("kube_job_status_active", "namespace", req.Namespace), consume: func(series []datasourcequery.Series) { appendWorkloads(result, series, "job_name", "Job") }},
		{name: "pods", expression: podVariableQuery(req.Filters), consume: func(series []datasourcequery.Series) { result.Pods = uniqueLabels(series, "pod") }},
	}

	for _, item := range queries {
		queryResult, queryErr := c.client.InstantSeriesQuery(ctx, ds, item.expression)
		if queryErr != nil {
			return nil, fmt.Errorf("query dashboard variable %s: %w", item.name, queryErr)
		}
		item.consume(queryResult.Series)
	}
	sort.Slice(result.Workloads, func(i, j int) bool {
		if result.Workloads[i].Kind == result.Workloads[j].Kind {
			return result.Workloads[i].Name < result.Workloads[j].Name
		}
		return result.Workloads[i].Kind < result.Workloads[j].Kind
	})
	return result, nil
}

func (c *controller) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	ds, err := c.loadDatasource(ctx, req.DatasourceID)
	if err != nil {
		return nil, err
	}
	start := time.Unix(req.Start, 0)
	end := time.Unix(req.End, 0)
	if !end.After(start) {
		return nil, apierrors.NewError(fmt.Errorf("end must be after start"), http.StatusBadRequest)
	}
	if end.Sub(start) > maximumRange {
		return nil, apierrors.NewError(fmt.Errorf("time range cannot exceed 7 days"), http.StatusBadRequest)
	}
	step := time.Duration(req.Step) * time.Second
	if step < time.Second {
		step = defaultStep
	}

	specs, err := selectPanels(req.PanelIDs)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	metricNames, catalogErr := c.client.MetricNames(ctx, ds)
	if catalogErr != nil {
		return nil, fmt.Errorf("load Prometheus metric catalog: %w", catalogErr)
	}

	results := make([]PanelResult, len(specs))
	semaphore := make(chan struct{}, queryParallel)
	var wg sync.WaitGroup
	for i, spec := range specs {
		i, spec := i, spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.queryPanel(ctx, ds, metricNames, spec, req.Filters, start, end, step, semaphore)
		}()
	}
	wg.Wait()
	return &QueryResponse{DatasourceID: req.DatasourceID, StartedAt: req.Start, EndedAt: req.End, Results: results}, nil
}

func (c *controller) queryPanel(ctx context.Context, ds *model.Datasource, metricNames map[string]struct{}, spec panelSpec, filters Filters, start, end time.Time, step time.Duration, semaphore chan struct{}) PanelResult {
	result := PanelResult{ID: spec.ID, Status: StatusSuccess, Series: []datasourcequery.Series{}}
	for _, metric := range spec.RequiredMetrics {
		if _, ok := metricNames[metric]; !ok {
			result.Status = StatusMetricMissing
			result.Message = "当前数据源未采集此面板所需指标"
			return result
		}
	}
	if spec.query == nil {
		result.Status = StatusMetricMissing
		result.Message = "当前数据源未采集此组件所需指标"
		return result
	}

	semaphore <- struct{}{}
	defer func() { <-semaphore }()
	var queryResult *datasourcequery.SeriesResult
	var err error
	expression := spec.query(filters)
	if spec.rangeQuery {
		queryResult, err = c.client.RangeSeriesQuery(ctx, ds, expression, start, end, step)
	} else {
		queryResult, err = c.client.InstantSeriesQuery(ctx, ds, expression)
	}
	if err != nil {
		result.Status = StatusError
		result.Message = err.Error()
		return result
	}
	result.Series = queryResult.Series
	if len(result.Series) == 0 {
		result.Status = StatusNoData
		result.Message = "当前筛选范围暂无数据"
	}
	return result
}

func selectPanels(ids []string) ([]panelSpec, error) {
	catalog := panelCatalog()
	if len(ids) == 0 {
		return catalog, nil
	}
	byID := make(map[string]panelSpec, len(catalog))
	for _, spec := range catalog {
		byID[spec.ID] = spec
	}
	result := make([]panelSpec, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, ok := seen[id]; ok {
			continue
		}
		spec, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown panel_id %q", id)
		}
		seen[id] = struct{}{}
		result = append(result, spec)
	}
	return result, nil
}

func variableSelector(metric, label, value string) string {
	if strings.TrimSpace(value) == "" {
		return metric
	}
	return fmt.Sprintf("%s{%s=%s}", metric, label, strconv.Quote(strings.TrimSpace(value)))
}

func podVariableQuery(filters Filters) string {
	podInfo := selector("kube_pod_info", filters, []string{"namespace", "node"})
	if strings.TrimSpace(filters.WorkloadName) == "" {
		return podInfo
	}
	if strings.EqualFold(strings.TrimSpace(filters.WorkloadKind), "Deployment") {
		replicaSetOwners := fmt.Sprintf(
			`max by(namespace,replicaset) (kube_replicaset_owner{owner_kind="Deployment",owner_name=%s})`,
			strconv.Quote(strings.TrimSpace(filters.WorkloadName)),
		)
		podReplicaSets := `max by(namespace,pod,replicaset) (label_replace(kube_pod_owner{owner_kind="ReplicaSet"}, "replicaset", "$1", "owner_name", "(.*)"))`
		return fmt.Sprintf("%s * on(namespace,pod) group_left(replicaset) (%s) * on(namespace,replicaset) group_left() (%s)", podInfo, podReplicaSets, replicaSetOwners)
	}
	owner := fmt.Sprintf("owner_name=%s", strconv.Quote(strings.TrimSpace(filters.WorkloadName)))
	if strings.TrimSpace(filters.WorkloadKind) != "" {
		owner += fmt.Sprintf(",owner_kind=%s", strconv.Quote(strings.TrimSpace(filters.WorkloadKind)))
	}
	return fmt.Sprintf("%s * on(namespace,pod) group_left(owner_kind,owner_name) kube_pod_owner{%s}", podInfo, owner)
}

func uniqueLabels(series []datasourcequery.Series, label string) []string {
	seen := map[string]struct{}{}
	for _, item := range series {
		value := strings.TrimSpace(item.Metric[label])
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func appendWorkloads(result *Variables, series []datasourcequery.Series, label, kind string) {
	seen := map[string]struct{}{}
	for _, item := range result.Workloads {
		seen[item.Kind+"\x00"+item.Name] = struct{}{}
	}
	for _, item := range series {
		name := strings.TrimSpace(item.Metric[label])
		if name == "" {
			continue
		}
		key := kind + "\x00" + name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result.Workloads = append(result.Workloads, WorkloadOption{Kind: kind, Name: name})
	}
}
