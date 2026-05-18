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

package auth

import (
	"context"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AuthGetter interface {
	Auth() Interface
}

type (
	Interface interface {
		CreateRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error
		DeleteRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error
		ListRBACPolicies(ctx context.Context, req *types.ListRBACPolicyRequest) ([]types.RBACPolicy, error)

		CreateGroupBinding(ctx context.Context, req *types.GroupBindingRequest) error
		DeleteGroupBinding(ctx context.Context, req *types.GroupBindingRequest) error
		ListGroupBindings(ctx context.Context, req *types.ListGroupBindingRequest) ([]types.RBACPolicy, error)
	}
)

type auth struct{}

func NewAuth(_ db.ShareDaoFactory) Interface {
	return &auth{}
}

func (a *auth) CreateRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error {
	return errors.ErrRBACDeprecated
}

func (a *auth) DeleteRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error {
	return errors.ErrRBACDeprecated
}

func (a *auth) ListRBACPolicies(ctx context.Context, req *types.ListRBACPolicyRequest) ([]types.RBACPolicy, error) {
	return []types.RBACPolicy{}, nil
}

func (a *auth) CreateGroupBinding(ctx context.Context, req *types.GroupBindingRequest) error {
	return errors.ErrRBACDeprecated
}

func (a *auth) DeleteGroupBinding(ctx context.Context, req *types.GroupBindingRequest) error {
	return errors.ErrRBACDeprecated
}

func (a *auth) ListGroupBindings(ctx context.Context, req *types.ListGroupBindingRequest) ([]types.RBACPolicy, error) {
	return []types.RBACPolicy{}, nil
}
