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
	"fmt"
	"net/http"

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
	ListRBACPolicies(ctx context.Context, req *types.ListRBACPolicyRequest) ([]types.RBACPolicy, error)
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
		return nil, errors.NewError(fmt.Errorf("user(%d) is not found", req.UserId), http.StatusBadRequest)
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
		klog.Errorf("failed to delete policy %v: %v", policy, err)
		return errors.ErrServerInternal
	}
	if !ok {
		return errors.ErrRBACPolicyNotFound
	}

	return nil
}

func (a *auth) ListRBACPolicies(ctx context.Context, req *types.ListRBACPolicyRequest) ([]types.RBACPolicy, error) {
	user, err := a.factory.User().Get(ctx, req.UserId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", req.UserId, err)
		return nil, errors.ErrServerInternal
	}
	if user == nil {
		return nil, errors.NewError(fmt.Errorf("user(%d) is not found", req.UserId), http.StatusBadRequest)
	}

	conds := []string{user.Name}
	if req.ObjectType != nil {
		conds = append(conds, req.ObjectType.String())
	}
	if req.SID != nil {
		conds = append(conds, *req.SID)
	}
	if req.Operation != nil {
		conds = append(conds, req.Operation.String())
	}

	policies, err := a.enforcer.GetFilteredNamedPolicy("p", 0, conds...)
	if err != nil {
		klog.Errorf("failed to list policies: %v", err)
	}

	rbacPolicies := make([]types.RBACPolicy, len(policies))
	for i, policy := range policies {
		rbacPolicies[i] = *model2Type(user.Name, policy)
	}
	return rbacPolicies, nil
}

func model2Type(username string, policy []string) *types.RBACPolicy {
	p := &types.RBACPolicy{
		Username: username,
	}

	switch len(policy) {
	case 2:
		p.ObjectType = model.ObjectType(policy[1])
	case 3:
		p.ObjectType = model.ObjectType(policy[1])
		p.StringID = policy[2]
	case 4:
		p.ObjectType = model.ObjectType(policy[1])
		p.StringID = policy[2]
		p.Operation = model.Operation(policy[3])
	}

	return p
}
