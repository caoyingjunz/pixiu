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

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (p *plan) CreateConfig(ctx context.Context, pid int64, req *types.CreatePlanConfigRequest) error {
	return nil
}

func (p *plan) UpdateConfig(ctx context.Context, pid int64, nodeId int64, req *types.CreatePlanConfigRequest) error {
	return nil
}

func (p *plan) DeleteConfig(ctx context.Context, pid int64, nodeId int64) error {
	return nil
}

func (p *plan) GetConfig(ctx context.Context, pid int64, nodeId int64) (*types.PlanConfig, error) {
	return nil, nil
}

func (p *plan) ListConfigs(ctx context.Context, pid int64) ([]types.PlanConfig, error) {
	return nil, nil
}
