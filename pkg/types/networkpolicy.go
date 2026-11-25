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

import (
	networkingv1 "k8s.io/api/networking/v1"
)

// NetworkPolicyCreateRequest 创建 NetworkPolicy 请求。
// 名称与命名空间分别取自 path 参数，不接受 body 覆盖，避免出现「body 里的 ns 和 path 不一致」的歧义。
type NetworkPolicyCreateRequest struct {
	// Name 网络策略名称，需符合 DNS-1123 subdomain 规范
	Name string `json:"name" binding:"required"`
	// Labels/Annotations 透传到对象元数据
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	// Spec 网络策略规则。
	// 注意：podSelector 为空选择器（既无 matchLabels 也无 matchExpressions）表示选中命名空间内全部 Pod，
	// 配合 policyTypes: [Ingress] 且无 ingress 规则即为「拒绝全部入站」，属高危配置，请确认语义后再提交。
	Spec networkingv1.NetworkPolicySpec `json:"spec"`
}

// NetworkPolicyUpdateRequest 更新 NetworkPolicy 请求，只允许修改元数据与 spec。
type NetworkPolicyUpdateRequest struct {
	Labels      map[string]string              `json:"labels,omitempty"`
	Annotations map[string]string              `json:"annotations,omitempty"`
	Spec        networkingv1.NetworkPolicySpec `json:"spec"`
	// ResourceVersion 并发控制。不传时取对象当前版本（后写覆盖先写）；
	// 传入过期版本时由 apiserver 返回 Conflict，避免静默覆盖他人修改。
	ResourceVersion string `json:"resource_version,omitempty"`
}
