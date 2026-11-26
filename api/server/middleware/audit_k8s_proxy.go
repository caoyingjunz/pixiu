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

package middleware

// 本文件只负责 K8s 代理链路（/pixiu/proxy/<cluster>/...）的路径解析与请求体提示。

import (
	"encoding/json"
	"net/http"
	"strings"

	auditlog "github.com/caoyingjunz/pixiu/pkg/audit"
)

// k8sKindMap 把 K8s 资源路径段（复数小写）映射为标准 Kind。
var k8sKindMap = map[string]string{
	"deployments":              "Deployment",
	"statefulsets":             "StatefulSet",
	"daemonsets":               "DaemonSet",
	"replicasets":              "ReplicaSet",
	"pods":                     "Pod",
	"services":                 "Service",
	"configmaps":               "ConfigMap",
	"secrets":                  "Secret",
	"ingresses":                "Ingress",
	"namespaces":               "Namespace",
	"nodes":                    "Node",
	"persistentvolumeclaims":   "PersistentVolumeClaim",
	"persistentvolumes":        "PersistentVolume",
	"horizontalpodautoscalers": "HorizontalPodAutoscaler",
	"cronjobs":                 "CronJob",
	"jobs":                     "Job",
	"events":                   "Event",
	"serviceaccounts":          "ServiceAccount",
	"roles":                    "Role",
	"rolebindings":             "RoleBinding",
	"clusterroles":             "ClusterRole",
	"clusterrolebindings":      "ClusterRoleBinding",
	"networkpolicies":          "NetworkPolicy",
	"storageclasses":           "StorageClass",
}

func k8sKindFromResource(resource string) string {
	if kind, ok := k8sKindMap[strings.ToLower(resource)]; ok {
		return kind
	}
	return resource
}

// proxyPathInfo 是从代理路径中解析出的 K8s 资源定位信息。
type proxyPathInfo struct {
	Cluster   string
	Name      string
	Namespace string
	Kind      string
}

// parseK8sProxyPathInfo 解析 /pixiu/proxy/<cluster>/api(s)/... 得到集群、命名空间、Kind 与名称。
func parseK8sProxyPathInfo(path string) proxyPathInfo {
	var info proxyPathInfo
	const proxyPrefix = "/pixiu/proxy/"
	if !strings.HasPrefix(path, proxyPrefix) {
		return info
	}
	rest := path[len(proxyPrefix):]
	parts := strings.SplitN(rest, "/", 2)
	if parts[0] == "" {
		return info
	}
	info.Cluster = parts[0]
	if len(parts) < 2 {
		return info
	}
	segments := strings.Split(strings.Trim(parts[1], "/"), "/")
	switch {
	case len(segments) >= 3 && segments[0] == "api":
		segments = segments[2:]
	case len(segments) >= 4 && segments[0] == "apis":
		segments = segments[3:]
	default:
		return info
	}
	// namespaces/<ns>/<resource>/<name> 结构，status/finalize 子资源不算命名空间层级。
	if len(segments) >= 3 && segments[0] == "namespaces" &&
		segments[2] != "status" && segments[2] != "finalize" {
		info.Namespace = segments[1]
		segments = segments[2:]
	}
	info.Kind = k8sKindFromResource(segments[0])
	if len(segments) > 1 {
		info.Name = segments[1]
	}
	return info
}

// proxyBodyHint 是从请求体推断出的补充信息。
type proxyBodyHint struct {
	Name      string
	Operation string
}

// inspectProxyBody 从请求体识别 restart/scale 语义，并补齐创建类请求的资源名。
func inspectProxyBody(raw []byte) proxyBodyHint {
	var hint proxyBodyHint
	if len(raw) == 0 {
		return hint
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		text := string(raw)
		if strings.Contains(text, "kubectl.kubernetes.io/restartedAt") {
			hint.Operation = auditlog.OperationRestart
		} else if strings.Contains(text, `"replicas"`) {
			hint.Operation = auditlog.OperationScale
		}
		return hint
	}
	if md, ok := obj["metadata"].(map[string]interface{}); ok {
		if name, ok := md["name"].(string); ok {
			hint.Name = name
		}
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if _, ok := spec["replicas"]; ok {
			hint.Operation = auditlog.OperationScale
		}
		if tmpl, ok := spec["template"].(map[string]interface{}); ok {
			if tmd, ok := tmpl["metadata"].(map[string]interface{}); ok {
				if anns, ok := tmd["annotations"].(map[string]interface{}); ok {
					if _, ok := anns["kubectl.kubernetes.io/restartedAt"]; ok {
						hint.Operation = auditlog.OperationRestart
					}
				}
			}
		}
	}
	return hint
}

// applyProxyBodyHint 只在 PATCH/PUT 上采用 restart/scale，避免创建 Deployment 时被 replicas 误判成扩缩容。
func applyProxyBodyHint(method string, rule *auditRule, proxy *proxyPathInfo, hint proxyBodyHint) {
	if rule == nil || proxy == nil {
		return
	}
	if hint.Name != "" && proxy.Name == "" {
		proxy.Name = hint.Name
	}
	if hint.Operation == "" {
		return
	}
	if method == http.MethodPatch || method == http.MethodPut {
		rule.Operation = hint.Operation
	}
}
