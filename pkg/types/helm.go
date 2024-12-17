/*
Copyright 2024 The Pixiu Authors.

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

type Release struct {
	Name    string                 `json:"name" binding:"required"`
	Chart   string                 `json:"chart" binding:"required"`
	Version string                 `json:"version" binding:"required"`
	Values  map[string]interface{} `json:"values"`
	Preview bool                   `json:"preview"`
}

type RepoId struct {
	Cluster string `uri:"cluster" binding:"required"`
	Id      int64  `uri:"id" binding:"required"`
}

type RepoName struct {
	Cluster string `uri:"cluster" binding:"required"`
	Name    string `uri:"name" binding:"required"`
}

type RepoURL struct {
	Url string `form:"url" binding:"required"`
}
type ChartValues struct {
	Chart   string `form:"chart" binding:"required"`
	Version string `form:"version" binding:"required"`
}
type RepoObjectMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type ReleaseHistory struct {
	Version int `form:"version"`
}

type RepoForm struct {
	Name                  string `json:"name" binding:"required"`
	URL                   string `json:"url" binding:"required"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	CertFile              string `json:"certFile"`
	KeyFile               string `json:"keyFile"`
	CAFile                string `json:"caFile"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
}

type RepoUpdateForm struct {
	Name                  string `json:"name" binding:"required"`
	URL                   string `json:"url" binding:"required"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	CertFile              string `json:"certFile"`
	KeyFile               string `json:"keyFile"`
	CAFile                string `json:"caFile"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
	ResourceVersion       *int64 `json:"resource_version" binding:"required"`
}

type HelmObjectMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
}
