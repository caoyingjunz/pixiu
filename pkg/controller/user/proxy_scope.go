/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

// parseK8sProxyPath 从 Kubernetes API path 中解析 namespace / resourceName。
// 示例：
//   /apis/apps/v1/namespaces/default/deployments/nginx -> ns=default, name=nginx
//   /api/v1/namespaces/default/pods -> ns=default, name=""
//   /api/v1/nodes/node-1 -> ns="", name=node-1（集群级资源，仅校验集群）
func parseK8sProxyPath(act string) (namespace, resourceName string) {
	parts := strings.Split(strings.Trim(act, "/"), "/")
	if len(parts) == 0 {
		return "", ""
	}

	for i, p := range parts {
		if p != "namespaces" {
			continue
		}
		if i+1 >= len(parts) {
			return "", ""
		}
		namespace = parts[i+1]
		// namespaces/{ns}/{resource}/{name}/...
		if i+3 < len(parts) {
			resourceName = parts[i+3]
		}
		return namespace, resourceName
	}

	// 集群级资源：/api/v1/{resource}/{name} 或 /apis/{g}/{v}/{resource}/{name}
	// 不做 resourceName 强校验（无命名空间上下文时仅靠 cluster 作用域）
	return "", ""
}

func matchRoleAPIScope(scopes []model.RoleAPIScope, cluster, namespace, resourceName string) bool {
	cluster = strings.TrimSpace(cluster)
	namespace = strings.TrimSpace(namespace)
	resourceName = strings.TrimSpace(resourceName)

	for i := range scopes {
		s := scopes[i]
		sc := strings.TrimSpace(s.Cluster)
		sn := strings.TrimSpace(s.Namespace)
		sr := strings.TrimSpace(s.ResourceName)
		if sr == "" {
			sr = "*"
		}
		if cluster != "" && sc != "*" && sc != cluster {
			continue
		}
		if namespace != "" && sn != "*" && sn != namespace {
			continue
		}
		if resourceName != "" && sr != "*" && sr != resourceName {
			continue
		}
		return true
	}
	return false
}
