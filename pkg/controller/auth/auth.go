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

	"github.com/casbin/casbin/v2"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type AuthGetter interface {
	Auth() Interface
}

type Interface interface {
	CreateRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error
	DeleteRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error
}

type auth struct {
	enforcer *casbin.SyncedEnforcer
	factory  db.ShareDaoFactory
}

func NewAuth(factory db.ShareDaoFactory, enforcer *casbin.SyncedEnforcer) Interface {
	return &auth{
		factory:  factory,
		enforcer: enforcer,
	}
}

// getPolicy returns the RBAC policy represented by the request body
func (a *auth) getPolicy(ctx context.Context, req *types.RBACPolicyRequest) ([]string, error) {
	user, err := a.factory.User().Get(ctx, req.UserId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", req.UserId, err)
		return nil, errors.ErrServerInternal
	}
	if user == nil {
		return nil, errors.ErrUserNotFound
	}

	return model.MakePolicy(user.Name, req.ObjectType, req.SID, req.Operation), nil
}

func (a *auth) CreateRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error {
	policy, err := a.getPolicy(ctx, req)
	if err != nil {
		return nil
	}

	ok, err := a.enforcer.AddPolicy(policy)
	if err != nil {
		klog.Errorf("failed to create policy %v: %v", policy, err)
		return errors.ErrServerInternal
	}
	if !ok {
		return errors.ErrRBACPolicyExists
	}

	return nil
}

func (a *auth) DeleteRBACPolicy(ctx context.Context, req *types.RBACPolicyRequest) error {
	policy, err := a.getPolicy(ctx, req)
	if err != nil {
		return nil
	}

	ok, err := a.enforcer.RemovePolicy(policy)
	if err != nil {
		klog.Error("failed to delete policy %v: %v", policy, err)
		return errors.ErrServerInternal
	}
	if !ok {
		return errors.ErrRBACPolicyNotFound
	}

	return nil
}
