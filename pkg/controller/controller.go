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

package controller

import (
	"github.com/casbin/casbin/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller/audit"
	"github.com/caoyingjunz/pixiu/pkg/controller/auth"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/controller/helm"
	"github.com/caoyingjunz/pixiu/pkg/controller/plan"
	"github.com/caoyingjunz/pixiu/pkg/controller/tenant"
	"github.com/caoyingjunz/pixiu/pkg/controller/user"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type PixiuInterface interface {
	cluster.ClusterGetter
	tenant.TenantGetter
	user.UserGetter
	plan.PlanGetter
	audit.AuditGetter
	auth.AuthGetter
	helm.HelmGetter
}

type pixiu struct {
	cc       config.Config
	factory  db.ShareDaoFactory
	enforcer *casbin.SyncedEnforcer
}

func (p *pixiu) Cluster() cluster.Interface { return cluster.NewCluster(p.cc, p.factory, p.enforcer) }
func (p *pixiu) Tenant() tenant.Interface   { return tenant.NewTenant(p.cc, p.factory) }
func (p *pixiu) User() user.Interface       { return user.NewUser(p.cc, p.factory, p.enforcer) }
func (p *pixiu) Plan() plan.Interface       { return plan.NewPlan(p.cc, p.factory) }
func (p *pixiu) Audit() audit.Interface     { return audit.NewAudit(p.cc, p.factory) }
func (p *pixiu) Auth() auth.Interface       { return auth.NewAuth(p.factory, p.enforcer) }
func (p *pixiu) Helm() helm.Interface       { return helm.NewHelm(p.factory) }

func New(cfg config.Config, f db.ShareDaoFactory, enforcer *casbin.SyncedEnforcer) PixiuInterface {
	return &pixiu{
		cc:       cfg,
		factory:  f,
		enforcer: enforcer,
	}
}
