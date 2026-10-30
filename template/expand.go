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

package template

// ExpandGlobals 集群扩容（add-node）渲染参数。
// 只包含 kubez-ansible add-node / bootstrap-servers 所需的最小集合，
// 不依赖任何部署计划配置。
type ExpandGlobals struct {
	KubeRelease      string
	APIServer        string
	Token            string
	CAHash           string
	ImageRepository  string
	NetworkInterface string
	SetHostname      bool
	RuntimeDataDir   string
	EnableCustomRepo bool
}

const ExpandGlobalsTemplate = `# Render below by Pixiu expander
---
kube_release: {{ .KubeRelease }}

kube_apiserver_address: {{ printf "%q" .APIServer }}
kube_admin_token: {{ printf "%q" .Token }}
kube_admin_token_ca_cert_hash: {{ printf "%q" .CAHash }}

{{- if ne .ImageRepository "" }}
image_repository: {{ printf "%q" .ImageRepository }}
app_image_repository: {{ printf "%q" .ImageRepository }}
{{- end }}

{{- if ne .NetworkInterface "" }}
network_interface: {{ printf "%q" .NetworkInterface }}
{{- end }}

{{- if ne .RuntimeDataDir "" }}
runtime_data_dir: {{ printf "%q" .RuntimeDataDir }}
{{- end }}

{{- if .SetHostname }}
set_hostname: "yes"
{{- else }}
set_hostname: "no"
{{- end }}

change_selinux: "no"

{{- if .EnableCustomRepo }}
enable_custom_repo: "yes"
{{- end }}
`
