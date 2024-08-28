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

package util

import (
	"context"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

func MakeDbOptions(ctx context.Context) (opts []db.Options) {
	exists, ids := httputils.GetIdRangeFromListReq(ctx)
	if exists {
		opts = append(opts, db.WithIDIn(ids...))
	}
	return
}

func SetIdRangeContext(c *gin.Context, enforcer *casbin.SyncedEnforcer, user *model.User, obj string) error {
	// group
	pp, err := enforcer.GetFilteredNamedGroupingPolicy("g", 0, user.Name)
	if err != nil {
		return err
	}
	bindings := make([]model.GroupBinding, len(pp))
	for i, p := range pp {
		copy(bindings[i][:], p)
	}
	if model.BindingToAdmin(bindings) {
		// This user is an admin/root, it's unnecessary to set object IDs list to context.
		return nil
	}

	ups, err := GetUserPolicies(enforcer, user, WithObjectType(model.ObjectType(obj)))
	if err != nil {
		return err
	}
	policies := make([]model.Policy, len(ups))
	for i, up := range ups {
		policies[i] = up
	}
	if all, ids := model.GetIdRangeFromPolicy(policies); !all {
		// Set a list of object IDs to context.
		httputils.SetIdRangeContext(c, ids)
	}
	// If policy with all operation(*) exists, it's unnecessary to set object IDs list to context.
	return nil
}

func GetGroupBindings(enforcer *casbin.SyncedEnforcer, policy model.GroupPolicy) ([]model.GroupBinding, error) {
	pp, err := enforcer.GetFilteredNamedGroupingPolicy("g", 1, policy.GetGroupName())
	if err != nil {
		return nil, err
	}
	bindings := make([]model.GroupBinding, len(pp))
	for i, p := range pp {
		copy(bindings[i][:], p)
	}
	return bindings, nil
}

type policyConditions struct {
	conds [4]*string
}

func newPolicyConditions(name string) *policyConditions {
	return &policyConditions{
		conds: [4]*string{&name},
	}
}

func (c *policyConditions) get() (conds []string) {
	for _, cond := range c.conds {
		if cond != nil {
			conds = append(conds, *cond)
		} else {
			break
		}
	}
	return
}

type PolicyCondition func(c *policyConditions)

func WithObjectType(t model.ObjectType) PolicyCondition {
	return func(c *policyConditions) {
		s := t.String()
		c.conds[1] = &s
	}
}

func WithStringID(sid string) PolicyCondition {
	return func(c *policyConditions) {
		c.conds[2] = &sid
	}
}

func WithOperation(op model.Operation) PolicyCondition {
	return func(c *policyConditions) {
		s := op.String()
		c.conds[3] = &s
	}
}

func GetUserPolicies(enforcer *casbin.SyncedEnforcer, user *model.User, conds ...PolicyCondition) ([]model.UserPolicy, error) {
	pc := newPolicyConditions(user.Name)
	for _, cond := range conds {
		cond(pc)
	}

	pp, err := enforcer.GetFilteredNamedPolicy("p", 0, pc.get()...)
	if err != nil {
		return nil, err
	}
	policies := make([]model.UserPolicy, len(pp))
	for i, p := range pp {
		copy(policies[i][:], p)
	}
	return policies, nil
}
