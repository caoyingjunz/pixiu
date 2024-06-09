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

const MultiModeTemplate = `# Render below by Pixiu engine
[docker-master]
{{- range .DockerMaster }}
{{- if eq .Auth.Type "password" }}
{{ .Name }} ansible_ssh_user={{ .Auth.Password.User }} ansible_ssh_pass={{ .Auth.Password.Password }}
{{- end }}
{{- if eq .Auth.Type "key" }}
{{ .Name }} ansible_ssh_user=root ansible_ssh_private_key_file={{ .Auth.Key.File }}
{{- end }}
{{- end }}

[docker-node]
{{- range .DockerNode }}
{{- if eq .Auth.Type "password" }}
{{ .Name }} ansible_ssh_user={{ .Auth.Password.User }} ansible_ssh_pass={{ .Auth.Password.Password }}
{{- end }}
{{- if eq .Auth.Type "key" }}
{{ .Name }} ansible_ssh_user=root ansible_ssh_private_key_file={{ .Auth.Key.File }}
{{- end }}
{{- end }}

[containerd-master]
{{- range .ContainerdMaster }}
{{- if eq .Auth.Type "password" }}
{{ .Name }} ansible_ssh_user={{ .Auth.Password.User }} ansible_ssh_pass={{ .Auth.Password.Password }}
{{- end }}
{{- if eq .Auth.Type "key" }}
{{ .Name }} ansible_ssh_user=root ansible_ssh_private_key_file={{ .Auth.Key.File }}
{{- end }}
{{- end }}

[containerd-node]
{{- range .ContainerdNode }}
{{- if eq .Auth.Type "password" }}
{{ .Name }} ansible_ssh_user={{ .Auth.Password.User }} ansible_ssh_pass={{ .Auth.Password.Password }}
{{- end }}
{{- if eq .Auth.Type "key" }}
{{ .Name }} ansible_ssh_user=root ansible_ssh_private_key_file={{ .Auth.Key.File }}
{{- end }}
{{- end }}

[storage]

# Don't change the bellow groups
[kube-master:children]
docker-master
containerd-master

[kube-node:children]
docker-node
containerd-node

[baremetal:children]
kube-master
kube-node
storage

[kubernetes:children]
kube-master
kube-node

[nfs-server:children]
storage

[haproxy:children]
kube-master
`
