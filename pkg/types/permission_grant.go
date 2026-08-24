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
	"fmt"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// ValidatePermissionGrant 校验授权类型与自定义规则。
// p_type 仅允许 0/1/2；管理员（2）仅超级管理员可授予；自定义规则禁止通配 verb 与 RBAC 提权。
func ValidatePermissionGrant(isRoot bool, pType int, rules []rbacv1.PolicyRule, namespaces []string) error {
	if pType < 0 || pType > 2 {
		return fmt.Errorf("p_type 仅支持 0（只读）、1（自定义）、2（管理员）")
	}
	if pType == 2 && !isRoot {
		return fmt.Errorf("仅超级管理员可授予集群管理员权限")
	}
	if pType != 1 {
		return nil
	}
	if len(namespaces) == 0 {
		return fmt.Errorf("自定义授权必须指定目标命名空间")
	}
	if len(rules) == 0 {
		return fmt.Errorf("自定义授权必须指定规则")
	}
	return validateCustomPolicyRules(rules)
}

func validateCustomPolicyRules(rules []rbacv1.PolicyRule) error {
	for i, rule := range rules {
		if containsStar(rule.Verbs) {
			return fmt.Errorf("自定义规则第 %d 条禁止使用通配动词 *", i+1)
		}
		if containsStar(rule.APIGroups) {
			return fmt.Errorf("自定义规则第 %d 条禁止使用通配 apiGroups *", i+1)
		}
		for _, g := range rule.APIGroups {
			if deniedAPIGroup(g) {
				return fmt.Errorf("自定义规则第 %d 条禁止授权 %s", i+1, g)
			}
		}
		for _, v := range rule.Verbs {
			if deniedVerb(v) {
				return fmt.Errorf("自定义规则第 %d 条禁止动词 %s", i+1, v)
			}
		}
		for _, r := range rule.Resources {
			if deniedResource(r) {
				return fmt.Errorf("自定义规则第 %d 条禁止资源 %s", i+1, r)
			}
		}
		if containsStar(rule.NonResourceURLs) {
			return fmt.Errorf("自定义规则第 %d 条禁止通配 nonResourceURLs", i+1)
		}
	}
	return nil
}

func containsStar(items []string) bool {
	for _, item := range items {
		if item == "*" {
			return true
		}
	}
	return false
}

func deniedAPIGroup(g string) bool {
	switch strings.ToLower(g) {
	case "rbac.authorization.k8s.io", "authorization.k8s.io", "authentication.k8s.io", "certificates.k8s.io":
		return true
	default:
		return false
	}
}

func deniedVerb(v string) bool {
	switch strings.ToLower(v) {
	case "impersonate", "bind", "escalate":
		return true
	default:
		return false
	}
}

func deniedResource(r string) bool {
	switch strings.ToLower(r) {
	case "clusterroles", "clusterrolebindings", "roles", "rolebindings",
		"serviceaccounts", "tokenreviews", "subjectaccessreviews",
		"selfsubjectaccessreviews", "localsubjectaccessreviews",
		"certificatesigningrequests", "secrets/exec":
		return true
	default:
		return false
	}
}
