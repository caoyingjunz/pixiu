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

package cluster

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// maxCronJobEventSelectors 限制单次聚合的 selector 数量，避免历史 Job 很多时放大 apiserver 查询。
const maxCronJobEventSelectors = 100

// calCronJobEventSelectors 聚合 CronJob 自身、其创建的子 Job 以及这些 Job 创建出的 Pod 的 fieldSelector。
// CronJob 本身很少直接产生事件，排查调度/创建失败主要依赖子 Job 与 Pod 的事件。
func (c *cluster) calCronJobEventSelectors(ctx context.Context, client *kubernetes.Clientset, namespace string, name string) ([]string, error) {
	cronJob, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get cronjob (%s/%s), err: %v", namespace, name, err)
		return nil, err
	}

	selectors := []string{c.makeFieldSelector(cronJob.UID, cronJob.Name, cronJob.Namespace, "CronJob")}

	jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return nil, err
	}
	// 优先聚合最近创建的 Job，保证达到上限时保留最新的事件来源。
	sort.SliceStable(jobs.Items, func(i, j int) bool {
		return jobs.Items[i].CreationTimestamp.After(jobs.Items[j].CreationTimestamp.Time)
	})

	for i := range jobs.Items {
		job := &jobs.Items[i]
		if !metav1.IsControlledBy(job, cronJob) {
			continue
		}
		if len(selectors) >= maxCronJobEventSelectors {
			return selectors, nil
		}
		selectors = append(selectors, c.makeFieldSelector(job.UID, job.Name, job.Namespace, "Job"))

		if job.Spec.Selector == nil {
			continue
		}
		jobSelector, selectorErr := metav1.LabelSelectorAsSelector(job.Spec.Selector)
		if selectorErr != nil || jobSelector.Empty() {
			continue
		}
		pods, listErr := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: jobSelector.String(),
			Limit:         500,
		})
		if listErr != nil {
			return nil, listErr
		}
		for j := range pods.Items {
			pod := &pods.Items[j]
			if !metav1.IsControlledBy(pod, job) {
				continue
			}
			if len(selectors) >= maxCronJobEventSelectors {
				return selectors, nil
			}
			selectors = append(selectors, c.makeFieldSelector(pod.UID, pod.Name, pod.Namespace, "Pod"))
		}
	}
	return selectors, nil
}
