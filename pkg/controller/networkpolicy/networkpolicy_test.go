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

package networkpolicy

import (
	"context"
	goerrors "errors"
	"net/http"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	pixiuerrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func newCreateRequest(name string) *types.NetworkPolicyCreateRequest {
	port := intstr.FromInt(80)
	return &types.NetworkPolicyCreateRequest{
		Name:   name,
		Labels: map[string]string{"app": "web"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{Port: &port},
					},
				},
			},
		},
	}
}

func TestBuildNetworkPolicy(t *testing.T) {
	policy, err := buildNetworkPolicy("default", newCreateRequest("allow-web"))
	if err != nil {
		t.Fatalf("构造对象失败: %v", err)
	}
	if policy.Name != "allow-web" || policy.Namespace != "default" {
		t.Fatalf("名称/命名空间不符: %s/%s", policy.Namespace, policy.Name)
	}
	if policy.Labels["app"] != "web" {
		t.Fatalf("标签未透传: %+v", policy.Labels)
	}
	if len(policy.Spec.PolicyTypes) != 1 || policy.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Fatalf("spec 未透传: %+v", policy.Spec)
	}

	// 非法名称 / 空命名空间要提前拦截，不要打到 apiserver 才报错
	if _, err = buildNetworkPolicy("default", newCreateRequest("Bad_Name")); err == nil {
		t.Fatalf("非法名称应当报错")
	}
	if _, err = buildNetworkPolicy("", newCreateRequest("allow-web")); err == nil {
		t.Fatalf("空命名空间应当报错")
	}
	if _, err = buildNetworkPolicy("default", nil); err == nil {
		t.Fatalf("空请求体应当报错")
	}
}

func TestNetworkPolicyCRUD(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx := context.Background()

	created, err := createNetworkPolicy(ctx, client, "default", newCreateRequest("allow-web"))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if created.Name != "allow-web" {
		t.Fatalf("创建结果不符: %s", created.Name)
	}

	// 重复创建应返回 409
	if _, err = createNetworkPolicy(ctx, client, "default", newCreateRequest("allow-web")); err == nil {
		t.Fatalf("重复创建应当报错")
	} else if code := errorCode(err); code != http.StatusConflict {
		t.Fatalf("重复创建应返回 409，实得 %d (%v)", code, err)
	}

	// 更新：spec 与标签被替换
	updated, err := updateNetworkPolicy(ctx, client, "default", "allow-web", &types.NetworkPolicyUpdateRequest{
		Labels: map[string]string{"app": "api"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Labels["app"] != "api" {
		t.Fatalf("标签未更新: %+v", updated.Labels)
	}
	if len(updated.Spec.PolicyTypes) != 1 || updated.Spec.PolicyTypes[0] != networkingv1.PolicyTypeEgress {
		t.Fatalf("spec 未更新: %+v", updated.Spec)
	}

	// 更新不存在的对象应返回 404
	if _, err = updateNetworkPolicy(ctx, client, "default", "not-exist", &types.NetworkPolicyUpdateRequest{}); err == nil {
		t.Fatalf("更新不存在的对象应当报错")
	} else if code := errorCode(err); code != http.StatusNotFound {
		t.Fatalf("更新不存在对象应返回 404，实得 %d", code)
	}

	// 删除后查询应返回 404
	if err = deleteNetworkPolicy(ctx, client, "default", "allow-web"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err = client.NetworkingV1().NetworkPolicies("default").Get(ctx, "allow-web", metav1.GetOptions{}); err == nil {
		t.Fatalf("删除后对象仍存在")
	}

	// 删除不存在的对象应返回 404
	if err = deleteNetworkPolicy(ctx, client, "default", "allow-web"); err == nil {
		t.Fatalf("删除不存在的对象应当报错")
	} else if code := errorCode(err); code != http.StatusNotFound {
		t.Fatalf("删除不存在对象应返回 404，实得 %d", code)
	}
}

func TestValidateHelpers(t *testing.T) {
	if err := validateNamespace("default"); err != nil {
		t.Fatalf("合法命名空间不应报错: %v", err)
	}
	if err := validateNamespace("Default"); err == nil {
		t.Fatalf("大写命名空间应当报错")
	}
	if err := validateObjectName("allow-web"); err != nil {
		t.Fatalf("合法名称不应报错: %v", err)
	}
	if err := validateObjectName("-allow"); err == nil {
		t.Fatalf("非法名称应当报错")
	}
}

// errorCode 取出 pixiu 错误里的 HTTP 码，非 pixiu 错误返回 0
func errorCode(err error) int {
	var e pixiuerrors.Error
	if goerrors.As(err, &e) {
		return e.Code
	}
	return 0
}
