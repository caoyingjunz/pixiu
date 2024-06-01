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

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (p *plan) createPlanTask(ctx context.Context, planId int64, step types.PlanStep) error {
	if _, err := p.factory.Plan().CreatTask(ctx, &model.Task{
		PlanId: planId,
		Step:   step,
	}); err != nil {
		klog.Errorf("failed to create plan(%d) task: %v", planId, err)
		return err
	}

	return nil
}

func (p *plan) deletePlanTask(ctx context.Context, planId int64) error {
	return nil
}

func (p *plan) syncPlanTask(ctx context.Context, planId int64, step int, message string) error {
	return nil
}
