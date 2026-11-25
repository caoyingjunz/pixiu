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

package types

import "time"

const (
	// DefaultSearchKindLimit 单个资源类型默认返回条数
	DefaultSearchKindLimit = 20
	// MaxSearchKindLimit 单个资源类型允许返回的最大条数
	MaxSearchKindLimit = 100
	// SearchListPageSize 单类资源单次从 apiserver 拉取的对象上限。
	// 关键字是「名称包含」语义，apiserver 不支持子串 fieldSelector，只能拉取后在服务端过滤，
	// 因此用该值给单次查询封顶，避免大集群全量 List 打爆 apiserver。
	SearchListPageSize = 500
	// MaxSearchClusters 一次请求允许搜索的集群数量上限。
	// 跨集群搜索是 N 个集群 × M 种资源的串行 List，必须限流，避免单个请求把控制面打穿。
	MaxSearchClusters = 10
)

// DefaultSearchKinds 未显式指定 kinds 时默认搜索的资源类型（高频、体量可控）。
// 需要覆盖 Events / PVC / Ingress 等排障常用类型时，由前端显式传 kinds，
// 或通过 GET /pixiu/search/kinds 拉取完整清单。
var DefaultSearchKinds = []string{"pods", "deployments", "services", "configmaps", "secrets"}

// SearchKindInfo 可搜索的资源类型描述，供前端渲染「资源类型」下拉。
type SearchKindInfo struct {
	Resource      string `json:"resource"`       // 复数资源名，搜索时作为 kinds 取值
	Kind          string `json:"kind"`           // 展示用 Kind
	APIVersion    string `json:"api_version"`    // 所属 group/version
	ClusterScoped bool   `json:"cluster_scoped"` // 是否集群级资源（忽略 namespace 过滤）
	Default       bool   `json:"default"`        // 是否属于未指定 kinds 时的默认集合
}

// SearchOptions 集群内资源搜索条件（全局搜索框 Ctrl+K 的服务端入参）。
type SearchOptions struct {
	// Keyword 资源名称关键字，大小写不敏感的子串匹配。
	// Keyword 与 LabelSelector 至少提供一个。
	Keyword string
	// Namespace 限定命名空间；为空表示全部命名空间，"all_namespaces" 视同为空。
	Namespace string
	// Kinds 限定资源类型（复数小写，如 pods/deployments），为空时取 DefaultSearchKinds。
	Kinds []string
	// LabelSelector 标签子串匹配，语义与集群资源列表页的 labelSelector 保持一致。
	LabelSelector string
	// Limit 单个资源类型的返回上限，<=0 取 DefaultSearchKindLimit，超过 MaxSearchKindLimit 按上限收敛。
	Limit int
}

// SearchItem 单条搜索结果。字段足够前端直接跳转到对应资源的详情页。
type SearchItem struct {
	Cluster    string            `json:"cluster"`
	Kind       string            `json:"kind"`
	APIVersion string            `json:"api_version"`
	Resource   string            `json:"resource"`
	Namespace  string            `json:"namespace,omitempty"`
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// SearchKindError 某类资源查询失败的说明。单类（或单集群）失败不影响其他结果返回，
// 典型场景是集群未安装对应 API（如老集群缺少某些 group）或该类型 RBAC 不足。
type SearchKindError struct {
	Cluster string `json:"cluster,omitempty"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// SearchResult 搜索结果汇总。
type SearchResult struct {
	// Cluster 单集群搜索时回填集群名；跨集群搜索时为空，见 Clusters。
	Cluster string `json:"cluster,omitempty"`
	// Clusters 本次搜索涉及的集群（按请求顺序去重后）。
	Clusters []string `json:"clusters"`
	// Total 本次返回的结果条数（已按类型截断）。
	Total int `json:"total"`
	// Truncated 存在结果被截断（单类型超过 limit，或拉取量触达 SearchListPageSize 上限）。
	Truncated   bool              `json:"truncated"`
	Items       []SearchItem      `json:"items"`
	Unavailable []SearchKindError `json:"unavailable,omitempty"`
}
