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

package search

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func newFakeClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-7d9f", Namespace: "default",
			Labels: map[string]string{"app": "nginx", "tier": "web"},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "redis-master-0", Namespace: "default",
			Labels: map[string]string{"app": "redis"},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-worker", Namespace: "prod",
			Labels: map[string]string{"app": "nginx"},
		}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx", Namespace: "default",
			Labels: map[string]string{"app": "nginx"},
		}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-canary", Namespace: "default",
			Labels: map[string]string{"app": "nginx"},
		}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-svc", Namespace: "default",
		}},
		&corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-svc", Namespace: "default",
		}},
		&corev1.Event{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-7d9f.17a1b2c3", Namespace: "default",
		}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-data", Namespace: "default",
		}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-ing", Namespace: "default",
		}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-migrate", Namespace: "default",
		}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name: "node-nginx-1", Labels: map[string]string{"role": "worker"},
		}},
	)
}

// runSearch 按集群 → 类型顺序执行搜索，返回汇总结果（与 controller.Search 的聚合部分等价）
func runSearch(t *testing.T, targets []clusterTarget, kinds []string, opt types.SearchOptions) *types.SearchResult {
	t.Helper()

	specs, err := resolveKinds(kinds)
	if err != nil {
		t.Fatalf("resolveKinds 失败: %v", err)
	}
	normalized, err := normalizeOptions(opt)
	if err != nil {
		t.Fatalf("normalizeOptions 失败: %v", err)
	}

	result := &types.SearchResult{Items: make([]types.SearchItem, 0)}
	searchClusterTargets(context.Background(), targets, specs, normalized, result)
	result.Total = len(result.Items)
	return result
}

func TestNormalizeOptions(t *testing.T) {
	tests := []struct {
		name    string
		in      types.SearchOptions
		wantErr bool
		want    types.SearchOptions
	}{
		{
			name:    "关键字与标签都为空应报错",
			in:      types.SearchOptions{},
			wantErr: true,
		},
		{
			name: "默认 limit 与 all_namespaces 归一化",
			in:   types.SearchOptions{Keyword: " nginx ", Namespace: "all_namespaces"},
			want: types.SearchOptions{Keyword: "nginx", Namespace: "", Limit: types.DefaultSearchKindLimit},
		},
		{
			name: "limit 超过上限收敛",
			in:   types.SearchOptions{Keyword: "nginx", Limit: 10000},
			want: types.SearchOptions{Keyword: "nginx", Limit: types.MaxSearchKindLimit},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOptions(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际未报错")
				}
				return
			}
			if err != nil {
				t.Fatalf("未期望报错: %v", err)
			}
			if got.Keyword != tt.want.Keyword || got.Namespace != tt.want.Namespace || got.Limit != tt.want.Limit {
				t.Fatalf("归一化结果不符: got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeClusters(t *testing.T) {
	// 空列表要报错
	if _, err := normalizeClusters(nil); err == nil {
		t.Fatalf("集群列表为空应当报错")
	}

	// 去空 + 去重 + 保序
	got, err := normalizeClusters([]string{" dev ", "", "prod", "dev"})
	if err != nil {
		t.Fatalf("未期望报错: %v", err)
	}
	if len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Fatalf("集群名归一化不符: %v", got)
	}

	// 超过上限要报错
	many := make([]string, 0, types.MaxSearchClusters+1)
	for i := 0; i <= types.MaxSearchClusters; i++ {
		many = append(many, fmt.Sprintf("c%d", i))
	}
	if _, err = normalizeClusters(many); err == nil {
		t.Fatalf("超过集群数量上限应当报错")
	}
}

func TestResolveKinds(t *testing.T) {
	// 不传 kinds 时使用默认集合
	specs, err := resolveKinds(nil)
	if err != nil {
		t.Fatalf("默认 kinds 不应报错: %v", err)
	}
	if len(specs) != len(types.DefaultSearchKinds) {
		t.Fatalf("默认 kinds 数量不符: got %d, want %d", len(specs), len(types.DefaultSearchKinds))
	}

	// 不支持的资源类型要明确报错，并给出可选值
	if _, err = resolveKinds([]string{"pods", "unknown-kind"}); err == nil {
		t.Fatalf("不支持的资源类型应当报错")
	} else if !strings.Contains(err.Error(), "pods") {
		t.Fatalf("错误信息应包含可选类型: %v", err)
	}

	// 重复类型去重
	specs, err = resolveKinds([]string{"pods", "PODS", " pods "})
	if err != nil {
		t.Fatalf("去重场景不应报错: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("重复类型应去重，got %d", len(specs))
	}
}

func TestKindsCatalog(t *testing.T) {
	kinds := (&controller{}).Kinds()
	if len(kinds) != len(kindRegistry) {
		t.Fatalf("类型清单数量不符: got %d, want %d", len(kinds), len(kindRegistry))
	}

	// 必须按资源名有序，前端下拉依赖稳定顺序
	for i := 1; i < len(kinds); i++ {
		if kinds[i-1].Resource > kinds[i].Resource {
			t.Fatalf("类型清单未按资源名排序: %s > %s", kinds[i-1].Resource, kinds[i].Resource)
		}
	}

	defaultCount := 0
	var events *types.SearchKindInfo
	for i := range kinds {
		if kinds[i].Default {
			defaultCount++
		}
		if kinds[i].Resource == "events" {
			events = &kinds[i]
		}
	}
	if defaultCount != len(types.DefaultSearchKinds) {
		t.Fatalf("默认类型标记数量不符: got %d, want %d", defaultCount, len(types.DefaultSearchKinds))
	}
	if events == nil || events.Kind != "Event" || events.APIVersion != "v1" || events.ClusterScoped {
		t.Fatalf("events 类型描述不符: %+v", events)
	}
}

func TestSearchClusterTargets(t *testing.T) {
	client := newFakeClient()
	targets := []clusterTarget{{name: "demo-cluster", client: client}}

	tests := []struct {
		name      string
		kinds     []string
		opt       types.SearchOptions
		wantNames []string
		wantKind  string
		truncated bool
	}{
		{
			name:      "按名称子串匹配 Pod",
			kinds:     []string{"pods"},
			opt:       types.SearchOptions{Keyword: "nginx", Limit: 10},
			wantNames: []string{"nginx-7d9f", "nginx-worker"},
			wantKind:  "Pod",
		},
		{
			name:      "namespace 限定后只返回该命名空间",
			kinds:     []string{"pods"},
			opt:       types.SearchOptions{Keyword: "nginx", Namespace: "prod", Limit: 10},
			wantNames: []string{"nginx-worker"},
			wantKind:  "Pod",
		},
		{
			name:      "名称大小写不敏感",
			kinds:     []string{"pods"},
			opt:       types.SearchOptions{Keyword: "NGINX", Limit: 10},
			wantNames: []string{"nginx-7d9f", "nginx-worker"},
			wantKind:  "Pod",
		},
		{
			name:      "完全匹配排在前缀匹配之前",
			kinds:     []string{"deployments"},
			opt:       types.SearchOptions{Keyword: "nginx", Limit: 10},
			wantNames: []string{"nginx", "nginx-canary"},
			wantKind:  "Deployment",
		},
		{
			name:      "标签子串匹配",
			kinds:     []string{"pods"},
			opt:       types.SearchOptions{LabelSelector: "tier=web", Limit: 10},
			wantNames: []string{"nginx-7d9f"},
			wantKind:  "Pod",
		},
		{
			name:      "limit 截断并标记 truncated",
			kinds:     []string{"pods"},
			opt:       types.SearchOptions{Keyword: "nginx", Limit: 1},
			wantNames: []string{"nginx-7d9f"},
			wantKind:  "Pod",
			truncated: true,
		},
		{
			name:      "集群级资源忽略 namespace 过滤",
			kinds:     []string{"nodes"},
			opt:       types.SearchOptions{Keyword: "nginx", Namespace: "prod", Limit: 10},
			wantNames: []string{"node-nginx-1"},
			wantKind:  "Node",
		},
		{
			name:      "Events 参与搜索",
			kinds:     []string{"events"},
			opt:       types.SearchOptions{Keyword: "nginx-7d9f", Limit: 10},
			wantNames: []string{"nginx-7d9f.17a1b2c3"},
			wantKind:  "Event",
		},
		{
			name:      "Endpoints / PVC / Ingress 参与搜索",
			kinds:     []string{"endpoints", "persistentvolumeclaims", "ingresses"},
			opt:       types.SearchOptions{Keyword: "nginx", Limit: 10},
			wantNames: []string{"nginx-svc", "nginx-data", "nginx-ing"},
		},
		{
			name:      "多类型混合搜索",
			kinds:     []string{"services", "jobs"},
			opt:       types.SearchOptions{Keyword: "nginx", Limit: 10},
			wantNames: []string{"nginx-svc", "nginx-migrate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runSearch(t, targets, tt.kinds, tt.opt)

			if got.Total != len(tt.wantNames) {
				t.Fatalf("Total 不符: got %d, want %d", got.Total, len(tt.wantNames))
			}
			if got.Truncated != tt.truncated {
				t.Fatalf("Truncated 不符: got %v, want %v", got.Truncated, tt.truncated)
			}
			if len(got.Unavailable) != 0 {
				t.Fatalf("不应出现 unavailable: %+v", got.Unavailable)
			}
			for i, name := range tt.wantNames {
				if got.Items[i].Name != name {
					t.Fatalf("第 %d 条结果名称不符: got %s, want %s", i, got.Items[i].Name, name)
				}
				if tt.wantKind != "" && got.Items[i].Kind != tt.wantKind {
					t.Fatalf("第 %d 条结果 Kind 不符: got %s, want %s", i, got.Items[i].Kind, tt.wantKind)
				}
				if got.Items[i].Cluster != "demo-cluster" {
					t.Fatalf("第 %d 条结果集群不符: %s", i, got.Items[i].Cluster)
				}
			}
		})
	}
}

func TestSearchClusterTargetsMultiCluster(t *testing.T) {
	clientA := newFakeClient()
	clientB := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "nginx-b", Namespace: "default"}},
	)
	targets := []clusterTarget{
		{name: "cluster-a", client: clientA},
		{name: "cluster-b", client: clientB},
	}

	got := runSearch(t, targets, []string{"pods"}, types.SearchOptions{Keyword: "nginx", Limit: 10})
	if got.Total != 3 {
		t.Fatalf("跨集群结果数量不符: got %d, want 3", got.Total)
	}
	// 结果按集群分组：先 cluster-a 的两条，再 cluster-b 的一条
	if got.Items[0].Cluster != "cluster-a" || got.Items[1].Cluster != "cluster-a" {
		t.Fatalf("前两条应来自 cluster-a: %+v", got.Items)
	}
	if got.Items[2].Cluster != "cluster-b" || got.Items[2].Name != "nginx-b" {
		t.Fatalf("第三条应来自 cluster-b: %+v", got.Items[2])
	}
}

// 单类资源查询失败（如 RBAC 不足）时，其余类型仍应正常返回结果
func TestSearchClusterTargetsPartialFailure(t *testing.T) {
	client := newFakeClient()
	client.PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("deployments is forbidden")
	})
	targets := []clusterTarget{{name: "demo-cluster", client: client}}

	got := runSearch(t, targets, []string{"pods", "deployments"}, types.SearchOptions{Keyword: "nginx", Limit: 10})
	if len(got.Unavailable) != 1 {
		t.Fatalf("应记录 1 条 unavailable: %+v", got.Unavailable)
	}
	if got.Unavailable[0].Kind != "deployments" || got.Unavailable[0].Cluster != "demo-cluster" {
		t.Fatalf("unavailable 内容不符: %+v", got.Unavailable[0])
	}
	for _, item := range got.Items {
		if item.Kind != "Pod" {
			t.Fatalf("失败类型的对象不应出现在结果里: %+v", item)
		}
	}
	if got.Total != 2 {
		t.Fatalf("Pod 结果数量不符: got %d, want 2", got.Total)
	}
}
