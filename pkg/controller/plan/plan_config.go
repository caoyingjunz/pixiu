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

package plan

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// ConfigInterface 计划配置子接口
type ConfigInterface interface {
	Get(ctx context.Context, planId int64) (*types.PlanConfig, error)
}

type planConfig struct {
	p *plan
}

func (p *plan) Config() ConfigInterface {
	return &planConfig{p: p}
}

func (p *plan) preCreateConfig(ctx context.Context, planId int64, req *types.CreatePlanConfigRequest) error {
	_, err := p.factory.Plan().Config().GetByPlan(ctx, planId)
	if err == nil {
		return fmt.Errorf("plan(%d) 配置已存在", planId)
	}

	return nil
}

// UpdateConfigIfNeeded
// 更新部署计划配置
func (p *plan) UpdateConfigIfNeeded(ctx context.Context, planId int64, req *types.UpdatePlanRequest) error {
	oldConfig, err := p.factory.Plan().Config().GetByPlan(ctx, planId)
	if err != nil {
		return errors.ErrServerInternal
	}
	newConfig := req.Config

	updates := make(map[string]interface{})
	if oldConfig.Region != newConfig.Region {
		updates["region"] = newConfig.Region
	}
	if oldConfig.OSImage != newConfig.OSImage {
		updates["os_image"] = newConfig.OSImage
	}

	newKubernetes, err := p.buildPlanKubernetesConfig(newConfig.Kubernetes)
	if err != nil {
		return err
	}
	if oldConfig.Kubernetes != newKubernetes {
		updates["kubernetes"] = newKubernetes
	}

	newNetwork, err := newConfig.Network.Marshal()
	if err != nil {
		return err
	}
	if oldConfig.Network != newNetwork {
		updates["network"] = newNetwork
	}

	newRuntime, err := newConfig.Runtime.Marshal()
	if err != nil {
		return err
	}
	if oldConfig.Runtime != newRuntime {
		updates["runtime"] = newRuntime
	}

	newComponent, err := newConfig.Component.Marshal()
	if err != nil {
		return err
	}
	if oldConfig.Component != newComponent {
		updates["component"] = newComponent
	}

	// 没有更新，则直接返回
	if len(updates) == 0 {
		return nil
	}
	if err = p.factory.Plan().Config().Update(ctx, oldConfig.Id, oldConfig.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update plan(%d) config: %v", planId, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (pc *planConfig) Get(ctx context.Context, pid int64) (*types.PlanConfig, error) {
	object, err := pc.p.factory.Plan().Config().GetByPlan(ctx, pid)
	if err != nil {
		klog.Errorf("failed to get plan(%d) config: %v", pid, err)
		return nil, errors.ErrServerInternal
	}

	return pc.p.modelConfig2Type(object)
}

func (p *plan) buildPlanKubernetesConfig(ks types.KubernetesSpec) (string, error) {
	if ks.EnablePublicIp {
		if len(ks.ApiServer) == 0 {
			return "", fmt.Errorf("启用 ApiServer 地址，但是未配置关联 IP")
		}
	} else {
		if len(ks.ApiServer) != 0 {
			ks.ApiServer = ""
		}
	}
	return ks.Marshal()
}

func (p *plan) buildPlanConfig(ctx context.Context, req *types.CreatePlanConfigRequest) (*model.Config, error) {
	kubeConfig, err := p.buildPlanKubernetesConfig(req.Kubernetes)
	if err != nil {
		return nil, err
	}
	networkConfig, err := req.Network.Marshal()
	if err != nil {
		return nil, err
	}
	runtimeConfig, err := req.Runtime.Marshal()
	if err != nil {
		return nil, err
	}
	componentConfig, err := req.Component.Marshal()
	if err != nil {
		return nil, err
	}

	return &model.Config{
		Region:     req.Region,
		OSImage:    req.OSImage,
		Kubernetes: kubeConfig,
		Network:    networkConfig,
		Runtime:    runtimeConfig,
		Component:  componentConfig,
	}, nil
}

func (p *plan) modelConfig2Type(o *model.Config) (*types.PlanConfig, error) {
	ks := &types.KubernetesSpec{}
	if err := ks.Unmarshal(o.Kubernetes); err != nil {
		return nil, err
	}
	ns := &types.NetworkSpec{}
	if err := ns.Unmarshal(o.Network); err != nil {
		return nil, err
	}
	rs := &types.RuntimeSpec{}
	if err := rs.Unmarshal(o.Runtime); err != nil {
		return nil, err
	}
	cs := &types.ComponentSpec{}
	if err := cs.Unmarshal(o.Component); err != nil {
		return nil, err
	}

	return &types.PlanConfig{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
		PlanId:     o.PlanId,
		Region:     o.Region,
		OSImage:    o.OSImage,
		Kubernetes: *ks,
		Network:    *ns,
		Runtime:    *rs,
		Component:  *cs,
	}, nil
}
