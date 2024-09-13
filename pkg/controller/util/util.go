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
	"github.com/caoyingjunz/pixiu/pkg/util"
)

func MakeDbOptions(ctx context.Context) (opts []db.Options) {
	exists, ids := httputils.GetIdRangeFromListReq(ctx)
	if exists {
		opts = append(opts, db.WithIDIn(ids...))
	}
	return
}

func SetIdRangeContext(c *gin.Context, enforcer *casbin.SyncedEnforcer, user *model.User, obj string) error {
	bindings, err := GetGroupBindings(enforcer, QueryWithUserName(user.Name))
	if err != nil {
		return err
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

type BindingQueryCondition func(c *policyConditions) (index int)

func QueryWithGroupName(name string) BindingQueryCondition {
	return func(c *policyConditions) int {
		c.conds[1] = &name
		return 1
	}
}

func QueryWithUserName(name string) BindingQueryCondition {
	return func(c *policyConditions) int {
		c.conds[0] = &name
		return 0
	}
}

func GetGroupBindings(enforcer *casbin.SyncedEnforcer, conds ...BindingQueryCondition) ([]model.GroupBinding, error) {
	var index int
	pc := &policyConditions{
		conds: [4]*string{},
	}
	for _, cond := range conds {
		i := cond(pc)
		index = util.Less(i, index)
	}

	rp, err := enforcer.GetFilteredNamedGroupingPolicy("g", index, pc.get()...)
	if err != nil {
		return nil, err
	}
	bindings := make([]model.GroupBinding, len(rp))
	for i, p := range rp {
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

	rp, err := enforcer.GetFilteredNamedPolicy("p", 0, pc.get()...)
	if err != nil {
		return nil, err
	}
	policies := make([]model.UserPolicy, len(rp))
	for i, p := range rp {
		_ = copy(policies[i][:], p)
	}
	return policies, nil
}

func GetGroupPolicy(enforcer *casbin.SyncedEnforcer, name string) (*model.GroupPolicy, error) {
	rp, err := enforcer.GetFilteredNamedPolicy("p", 0, name)
	if err != nil {
		return nil, err
	}
	if len(rp) == 0 {
		return nil, nil
	}
	policy := model.GroupPolicy{}
	_ = copy(policy[:], rp[0])
	return &policy, nil
}
