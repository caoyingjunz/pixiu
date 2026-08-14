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

import "github.com/caoyingjunz/pixiu/pkg/datasource/query"

const (
	StatusSuccess       = "success"
	StatusNoData        = "no_data"
	StatusMetricMissing = "metric_missing"
	StatusError         = "error"
)

type Section struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Icon     string   `json:"icon"`
	Children []string `json:"children,omitempty"`
}

type PanelDefinition struct {
	ID              string   `json:"id"`
	Section         string   `json:"section"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Kind            string   `json:"kind"`
	Unit            string   `json:"unit,omitempty"`
	Span            int      `json:"span"`
	RequiredMetrics []string `json:"required_metrics,omitempty"`
}

type Definition struct {
	Sections []Section         `json:"sections"`
	Panels   []PanelDefinition `json:"panels"`
}

type Filters struct {
	Namespace    string `json:"namespace" form:"namespace"`
	Node         string `json:"node" form:"node"`
	WorkloadKind string `json:"workload_kind" form:"workload_kind"`
	WorkloadName string `json:"workload_name" form:"workload_name"`
	Pod          string `json:"pod" form:"pod"`
}

type VariableRequest struct {
	DatasourceID int64 `form:"datasource_id" binding:"required"`
	Filters
}

type WorkloadOption struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Variables struct {
	Namespaces []string         `json:"namespaces"`
	Nodes      []string         `json:"nodes"`
	Workloads  []WorkloadOption `json:"workloads"`
	Pods       []string         `json:"pods"`
}

type QueryRequest struct {
	DatasourceID int64    `json:"datasource_id" binding:"required"`
	PanelIDs     []string `json:"panel_ids"`
	Start        int64    `json:"start" binding:"required"`
	End          int64    `json:"end" binding:"required"`
	Step         int64    `json:"step"`
	Filters      Filters  `json:"filters"`
}

type PanelResult struct {
	ID      string         `json:"id"`
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Series  []query.Series `json:"series"`
}

type QueryResponse struct {
	DatasourceID int64         `json:"datasource_id"`
	StartedAt    int64         `json:"started_at"`
	EndedAt      int64         `json:"ended_at"`
	Results      []PanelResult `json:"results"`
}
