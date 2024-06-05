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

const HostTemplate = `# Render below by Pixiu engine
127.0.0.1	localhost
::1       localhost localhost.localdomain localhost6 localhost6.localdomain6
{{- range .Nodes }}
{{ .Ip }}  {{ .Name }}
{{- end }}
`
