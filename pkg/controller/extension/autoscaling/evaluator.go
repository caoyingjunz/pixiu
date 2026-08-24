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
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

// EvaluateOnce 扫描所有启用中的定时扩缩容规则，执行到期任务并记录历史。
// 由 jobmanager 周期调用；触发判定以 last_fire_time 推进 + 乐观锁认领，
// 保证多实例部署时同一触发点至多执行一次。
func (c *controller) EvaluateOnce(ctx context.Context) error {
	rules, err := c.factory.CronHpa().List(ctx, db.WithCronHpaStatus(model.CronHpaStatusActive))
	if err != nil {
		return err
	}
	for i := range rules {
		c.evaluateRule(ctx, &rules[i])
	}
	return nil
}

func (c *controller) evaluateRule(ctx context.Context, rule *model.CronHpa) {
	jobs, err := parseJobs(rule.Jobs)
	if err != nil {
		klog.Errorf("[CronHpa] 规则 %s(%d) 任务数据解析失败: %v", rule.Name, rule.Id, err)
		return
	}
	now := time.Now()
	excluded := excludedToday(rule.ExcludeDates, now)

	// 计算本轮到期的任务；宕机错过的触发只补最近一次，避免逐个补跑造成抖动
	dueIndexes := make([]int, 0, len(jobs))
	fireTimes := make([]time.Time, len(jobs))
	for i := range jobs {
		fire, due, dueErr := nextDueFireTime(&jobs[i], now)
		if dueErr != nil {
			klog.Errorf("[CronHpa] 规则 %s(%d) 任务 %s 调度解析失败: %v", rule.Name, rule.Id, jobs[i].Name, dueErr)
			continue
		}
		if !due {
			continue
		}
		fireTimes[i] = fire
		dueIndexes = append(dueIndexes, i)
	}
	if len(dueIndexes) == 0 {
		return
	}

	// 先推进 last_fire_time 并以乐观锁认领，认领成功后才执行
	for _, i := range dueIndexes {
		fire := fireTimes[i]
		jobs[i].LastFireTime = &fire
	}
	jobsData, err := json.Marshal(jobs)
	if err != nil {
		klog.Errorf("[CronHpa] 规则 %s(%d) 任务序列化失败: %v", rule.Name, rule.Id, err)
		return
	}
	claimCtx, claimCancel := context.WithTimeout(ctx, dbOpTimeout)
	claimed, err := c.factory.CronHpa().ClaimAndUpdate(claimCtx, rule.Id, rule.ResourceVersion,
		map[string]interface{}{"jobs": string(jobsData)})
	claimCancel()
	if err != nil {
		klog.Errorf("[CronHpa] 规则 %s(%d) 认领失败: %v", rule.Name, rule.Id, err)
		return
	}
	if !claimed {
		// 已被其他实例处理
		klog.V(2).Infof("[CronHpa] 规则 %s(%d) 本轮触发已被其他实例认领", rule.Name, rule.Id)
		return
	}

	// 执行到期任务
	cluster, clusterErr := c.getCluster(ctx, rule.ClusterName)
	var cs *client.ClusterSet
	if clusterErr == nil {
		cs, clusterErr = buildClusterSet(cluster)
	}
	execCtx, cancel := context.WithTimeout(ctx, kubeOpTimeout)
	defer cancel()

	for _, i := range dueIndexes {
		job := &jobs[i]
		switch {
		case clusterErr != nil:
			c.recordHistory(ctx, rule, job, fireTimes[i], 0, job.TargetSize,
				types.CronHpaHistoryResultFailed, "集群不可用: "+clusterErr.Error())
			job.State = types.CronHpaJobStateFailed
			job.Message = "集群不可用: " + clusterErr.Error()
		case excluded:
			c.recordHistory(ctx, rule, job, fireTimes[i], 0, job.TargetSize,
				types.CronHpaHistoryResultSkipped, "命中排除日期，跳过执行")
			job.State = types.CronHpaJobStateSkipped
			job.Message = "命中排除日期，跳过执行"
		default:
			previous, message, execErr := executeJob(execCtx, cs, rule, job)
			if execErr != nil {
				c.recordHistory(ctx, rule, job, fireTimes[i], previous, job.TargetSize,
					types.CronHpaHistoryResultFailed, execErr.Error())
				job.State = types.CronHpaJobStateFailed
				job.Message = execErr.Error()
				klog.Errorf("[CronHpa] 规则 %s(%d) 任务 %s 执行失败: %v", rule.Name, rule.Id, job.Name, execErr)
			} else {
				c.recordHistory(ctx, rule, job, fireTimes[i], previous, job.TargetSize,
					types.CronHpaHistoryResultSucceed, message)
				job.State = types.CronHpaJobStateSucceed
				job.Message = message
				klog.Infof("[CronHpa] 规则 %s(%d) 任务 %s 执行成功: %s", rule.Name, rule.Id, job.Name, message)
			}
		}
	}

	// 回写各任务最近执行状态（尽力而为，不影响本轮执行结果）
	if jobsData, err = json.Marshal(jobs); err == nil {
		wbCtx, wbCancel := context.WithTimeout(ctx, dbOpTimeout)
		err = c.factory.CronHpa().InternalUpdate(wbCtx, rule.Id, map[string]interface{}{"jobs": string(jobsData)})
		wbCancel()
		if err != nil {
			klog.Errorf("[CronHpa] 规则 %s(%d) 状态回写失败: %v", rule.Name, rule.Id, err)
		}
	}
}

// nextDueFireTime 判定任务是否存在到期触发：
// 以 LastFireTime 为基准计算下一个触发点，若已过期则推进至最近一次应触发时刻（只补最近一次）。
func nextDueFireTime(job *types.CronHpaJob, now time.Time) (time.Time, bool, error) {
	// runOnce 任务执行过一次（状态离开 Submitted）后不再触发
	if job.RunOnce && job.State != types.CronHpaJobStateSubmitted {
		return time.Time{}, false, nil
	}
	if job.LastFireTime == nil {
		return time.Time{}, false, nil
	}
	sched, err := cron.ParseStandard(job.Schedule)
	if err != nil {
		return time.Time{}, false, err
	}
	fire := sched.Next(*job.LastFireTime)
	if fire.After(now) {
		return time.Time{}, false, nil
	}
	skipped := 0
	for step := 0; step < catchupMaxSteps; step++ {
		next := sched.Next(fire)
		if next.After(now) {
			break
		}
		fire = next
		skipped++
	}
	if skipped > 0 {
		// 中间触发点被跳过（如评估轮被阻塞/宕机），仅补最近一次，需显式告警
		klog.Warningf("[CronHpa] 任务 %s 补跑时跳过 %d 个过期触发点，仅执行最近一次 %s", job.Name, skipped, fire.Format(time.RFC3339))
	}
	return fire, true, nil
}

// excludedToday 判断当天是否命中任一排除日期（cron 表达式，最小粒度为天）
func excludedToday(excludeDates string, now time.Time) bool {
	if excludeDates == "" || excludeDates == "null" {
		return false
	}
	var patterns []string
	if err := json.Unmarshal([]byte(excludeDates), &patterns); err != nil || len(patterns) == 0 {
		return false
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	for _, pattern := range patterns {
		sched, err := cron.ParseStandard(pattern)
		if err != nil {
			continue
		}
		if next := sched.Next(dayStart.Add(-time.Second)); next.Before(dayEnd) {
			return true
		}
	}
	return false
}

func parseJobs(data string) ([]types.CronHpaJob, error) {
	var jobs []types.CronHpaJob
	if err := json.Unmarshal([]byte(data), &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c *controller) recordHistory(ctx context.Context, rule *model.CronHpa, job *types.CronHpaJob,
	scheduled time.Time, previous, desired int32, result, message string) {
	history := &model.CronHpaHistory{
		CronHpaId:        rule.Id,
		JobName:          job.Name,
		ScheduledTime:    scheduled,
		ExecutedAt:       time.Now(),
		PreviousReplicas: previous,
		DesiredReplicas:  desired,
		Result:           result,
		Message:          message,
	}
	histCtx, histCancel := context.WithTimeout(ctx, dbOpTimeout)
	err := c.factory.CronHpa().AppendHistory(histCtx, history)
	histCancel()
	if err != nil {
		klog.Errorf("[CronHpa] 规则 %s(%d) 执行历史落库失败: %v", rule.Name, rule.Id, err)
	}
}
