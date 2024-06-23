/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

const GlobalsTemplate = `# Render below by Pixiu
---
{{- if .Kubernetes.EnableHA }}
enable_kubernetes_ha: "yes"
{{- end }}

{{- if ne .Kubernetes.ApiServer "" }}
kube_vip_address: "{{ .Kubernetes.ApiServer }}"
{{- end }}

kube_release: {{ .Kubernetes.KubernetesVersion }}

cluster_cidr: "{{ .Network.PodNetwork }}"
service_cidr: "{{ .Network.ServiceNetwork }}"

network_interface: "{{ .Network.NetworkInterface }}"

{{- if eq .Network.Cni "calico" }}
enable_calico: "yes"
{{- end }}

enable_nfs: "no"
`
