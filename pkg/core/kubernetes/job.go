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

package kubernetes

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type JobsGetter interface {
	Jobs(cloud string) JobInterface
}

type JobInterface interface {
	List(ctx context.Context, listOptions types.ListOptions) ([]batchv1.Job, error)
}

type jobs struct {
	client *kubernetes.Clientset
	cloud  string
}

func NewJobs(c *kubernetes.Clientset, cloud string) *jobs {
	return &jobs{
		client: c,
		cloud:  cloud,
	}
}

func (c *jobs) List(ctx context.Context, listOptions types.ListOptions) ([]batchv1.Job, error) {
	if c.client == nil {
		return nil, clientError
	}
	job, err := c.client.BatchV1().
		Jobs(listOptions.Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", listOptions.Namespace, err)
		return nil, err
	}

	return job.Items, nil
}
