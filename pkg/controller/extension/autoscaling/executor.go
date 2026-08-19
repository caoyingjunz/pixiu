/*
Copyright 2026 The Pixiu Authors.

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

package autoscaling

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// executeJob 执行一次定时扩缩动作，返回执行前副本数与执行结果描述：
//   - 目标为 Deployment/StatefulSet：通过 scale 子资源直接修改副本数；
//   - 目标为 HPA（兼容模式）：按兼容规则调整 HPA 的 min/max，不直接写副本数，
//     副本伸缩仍由 HPA 按指标完成，实现"定时 + 指标"共存。
func executeJob(ctx context.Context, cs *client.ClusterSet, rule *model.CronHpa, job *types.CronHpaJob) (int32, string, error) {
	switch rule.TargetKind {
	case types.CronHpaTargetKindHpa:
		return adjustHpaBounds(ctx, cs, rule.Namespace, rule.TargetName, job.TargetSize)
	case types.CronHpaTargetKindDeployment, types.CronHpaTargetKindStatefulSet:
		return scaleWorkload(ctx, cs, rule, job.TargetSize)
	default:
		return 0, "", fmt.Errorf("不支持的目标类型 %s", rule.TargetKind)
	}
}

// scaleWorkload 直接扩缩工作负载副本数
func scaleWorkload(ctx context.Context, cs *client.ClusterSet, rule *model.CronHpa, desired int32) (int32, string, error) {
	scale, err := getScale(ctx, cs, rule)
	if err != nil {
		return 0, "", err
	}
	previous := scale.Spec.Replicas
	message := fmt.Sprintf("current replicas:%d, desired replicas:%d", previous, desired)
	if previous == desired {
		return previous, message + "，已达目标值，无需变更", nil
	}
	scale.Spec.Replicas = desired
	if err = updateScale(ctx, cs, rule, scale); err != nil {
		return previous, "", err
	}
	// 按实际方向描述：扩容/缩容
	action := "扩容成功"
	if desired < previous {
		action = "缩容成功"
	}
	return previous, message + "，" + action, nil
}

func getScale(ctx context.Context, cs *client.ClusterSet, rule *model.CronHpa) (*autoscalingv1.Scale, error) {
	switch rule.TargetKind {
	case types.CronHpaTargetKindDeployment:
		return cs.Client.AppsV1().Deployments(rule.Namespace).GetScale(ctx, rule.TargetName, metav1.GetOptions{})
	case types.CronHpaTargetKindStatefulSet:
		return cs.Client.AppsV1().StatefulSets(rule.Namespace).GetScale(ctx, rule.TargetName, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("不支持的目标类型 %s", rule.TargetKind)
	}
}

func updateScale(ctx context.Context, cs *client.ClusterSet, rule *model.CronHpa, scale *autoscalingv1.Scale) error {
	switch rule.TargetKind {
	case types.CronHpaTargetKindDeployment:
		_, err := cs.Client.AppsV1().Deployments(rule.Namespace).UpdateScale(ctx, rule.TargetName, scale, metav1.UpdateOptions{})
		return err
	case types.CronHpaTargetKindStatefulSet:
		_, err := cs.Client.AppsV1().StatefulSets(rule.Namespace).UpdateScale(ctx, rule.TargetName, scale, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("不支持的目标类型 %s", rule.TargetKind)
	}
}

// adjustHpaBounds 兼容模式：CronHPA 作为副本数的动态下限，语义对齐阿里云 CronHPA 兼容规则：
// - desired > maxReplicas：min/max 同时抬到 desired；
// - desired > 当前副本：抬升 minReplicas，HPA 随即扩到 desired；
// - desired <= 当前副本：仅下调 minReplicas，由 HPA 按指标自行缩容，不直接动副本；
// - desired == 当前副本：不做调整。
func adjustHpaBounds(ctx context.Context, cs *client.ClusterSet, namespace, name string, desired int32) (int32, string, error) {
	hpa, err := cs.Client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, "", err
	}

	current := hpa.Status.CurrentReplicas
	oldMin := int32(1)
	if hpa.Spec.MinReplicas != nil {
		oldMin = *hpa.Spec.MinReplicas
	}
	oldMax := hpa.Spec.MaxReplicas

	newMin, newMax := oldMin, oldMax
	switch {
	case desired == current:
		// 目标等于当前副本，min/max 保持不变
	case desired > oldMax:
		newMin, newMax = desired, desired
	case desired > current:
		newMin = desired
	default:
		// desired <= current：只下调下限，缩容交由 HPA 指标决策
		newMin = desired
	}

	message := fmt.Sprintf("current replicas:%d, desired replicas:%d", current, desired)
	if newMin == oldMin && newMax == oldMax {
		return current, message + "，HPA min/max 无需调整", nil
	}
	hpa.Spec.MinReplicas = &newMin
	hpa.Spec.MaxReplicas = newMax
	if _, err = cs.Client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, hpa, metav1.UpdateOptions{}); err != nil {
		return current, "", err
	}
	return current, fmt.Sprintf("%s，HPA min/max 由 %d/%d 调整为 %d/%d", message, oldMin, oldMax, newMin, newMax), nil
}
