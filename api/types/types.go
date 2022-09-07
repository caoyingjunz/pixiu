/*
Copyright 2021 The Pixiu Authors.

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

type IdOptions struct {
	Id int64 `uri:"id" binding:"required"`
}

type CloudOptions struct {
	CloudName string `uri:"cloud_name" binding:"required"`
}

type ListOptions struct {
	CloudName string `uri:"cloud_name" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
}

type GetOrDeleteOptions struct {
	ListOptions

	ObjectName string `uri:"object_name" binding:"required"`
}

type GetOrCreateOptions struct {
	ListOptions

	ObjectName string `uri:"object_name" binding:"required"`
}

type CreateOptions struct {
	Replicas      int32  `json:"replicas"`
	ImageName     string `json:"image_name" binding:"required"`
	Image         string `json:"image" binding:"required"`
	ContainerPort int32  `json:"container_port" binding:"required"`
	//Lable         map[string]string `json:"lable"` //  应该这样写  不知如何传参以及取值
	LableName string   `json:"lable_name"`
	Lable     string   `json:"lable"`
	Command   []string `json:"command"`
	//ImagePullPolicy导入问题
	ImagePullPolicy string            `json:"image_pull_policy"`
	PortsName       string            `json:"ports_name" binding:"required"`
	NodeSelector    map[string]string `json:"node_selector,omitempty"`
}

type Demo struct {
	Id              int64  `json:"id"`
	ResourceVersion int64  `json:"resource_version"`
	Name            string `json:"name"`
}

type Cicd struct {
	Name     string `json:"name,omitempty"`
	OldName  string `json:"oldName,omitempty"`
	NewName  string `json:"newName,omitempty"`
	ViewName string `json:"viewname,omitempty"`
	Version  string `json:"version,omitempty"`
}

type User struct {
	Id              int64  `json:"id"`
	ResourceVersion int64  `json:"resource_version"`
	Name            string `json:"name"`
	Password        string `json:"password"`
	Status          int8   `json:"status"`
	Role            string `json:"role"`
	Email           string `json:"email"`
	Description     string `json:"description"`

	TimeSpec
}

type Password struct {
	UserId          int64  `json:"user_id"`
	OriginPassword  string `json:"origin_password"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

type Cloud struct {
	Id              int64  `json:"id"`
	ResourceVersion int64  `json:"resource_version"`
	Name            string `json:"name"`
	Status          int    `json:"status"`
	KubeConfig      []byte `json:"kube_config"`
	Description     string `json:"description"`

	TimeSpec
}

// TimeSpec 通用时间规格
type TimeSpec struct {
	GmtCreate   interface{} `json:"gmt_create,omitempty"`
	GmtModified interface{} `json:"gmt_modified,omitempty"`
}
