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

import "github.com/caoyingjunz/pixiu/pkg/db/model"

type (
	// LoginRequest is the request body struct for user login.
	LoginRequest struct {
		Name     string `json:"name" binding:"required"`     // required
		Password string `json:"password" binding:"required"` // required
	}

	CreateUserRequest struct {
		Name     string `json:"name" binding:"required"`              // required
		Password string `json:"password" binding:"required,password"` // required
		// Status      model.UserStatus `json:"status" binding:"omitempty"`
		Role        model.UserRole `json:"role" binding:"omitempty,oneof=0 1 2"` // optional
		Email       string         `json:"email" binding:"omitempty,email"`      // optional
		Description string         `json:"description" binding:"omitempty"`      // optional
	}

	CreateClusterRequest struct {
		Name        string            `json:"name" binding:"omitempty"`                   // optional
		AliasName   string            `json:"alias_name" binding:"omitempty"`             // optional
		Type        model.ClusterType `json:"cluster_type" binding:"omitempty,oneof=0 1"` // optional
		KubeConfig  string            `json:"kube_config" binding:"required"`             // required
		Description string            `json:"description" binding:"omitempty"`            // optional
		Protected   bool              `json:"protected" binding:"omitempty"`              // optional
	}

	UpdateClusterRequest struct {
		AliasName   *string `json:"alias_name" binding:"omitempty"`  // optional
		Description *string `json:"description" binding:"omitempty"` // optional
		// TODO: put resource version in a common struct for updating request only
		ResourceVersion int64 `json:"resource_version" binding:"required"` // required
	}

	ProtectClusterRequest struct {
		ResourceVersion int64 `json:"resource_version" binding:"required"` // required
		Protected       bool  `json:"protected" binding:"omitempty"`       // optional
	}

	CreateTenantRequest struct {
		Name        string  `json:"name" binding:"required"`         // required
		Description *string `json:"description" binding:"omitempty"` // optional
	}
)
