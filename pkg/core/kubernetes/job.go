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

	"github.com/caoyingjunz/gopixiu/api/meta"
	pixiuerrors "github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type JobsGetter interface {
	Jobs(cloud string) JobInterface
}

type JobInterface interface {
	Create(ctx context.Context, job *batchv1.Job) error
	Update(ctx context.Context, job *batchv1.Job) error
	Delete(ctx context.Context, deleteOptions meta.DeleteOptions) error
	Get(ctx context.Context, getOptions meta.GetOptions) (*batchv1.Job, error)
	List(ctx context.Context, listOptions meta.ListOptions) ([]batchv1.Job, error)
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

func (c *jobs) Create(ctx context.Context, job *batchv1.Job) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.BatchV1().
		Jobs(job.Namespace).
		Create(ctx, job, metav1.CreateOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s namespace %s: %v", c.cloud, job.Namespace, err)

		return err
	}

	return nil
}

func (c *jobs) Update(ctx context.Context, job *batchv1.Job) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if _, err := c.client.BatchV1().
		Jobs(job.Namespace).
		Update(ctx, job, metav1.UpdateOptions{}); err != nil {
		log.Logger.Errorf("failed to update %s statefulSet: %v", c.cloud, err)
		return err
	}

	return nil
}

func (c *jobs) Delete(ctx context.Context, deleteOptions meta.DeleteOptions) error {
	if c.client == nil {
		return pixiuerrors.ErrCloudNotRegister
	}
	if err := c.client.BatchV1().
		Jobs(deleteOptions.Namespace).
		Delete(ctx, deleteOptions.ObjectName, metav1.DeleteOptions{}); err != nil {
		log.Logger.Errorf("failed to delete %s deployment: %v", deleteOptions.Namespace, err)
		return err
	}

	return nil
}

func (c *jobs) Get(ctx context.Context, getOptions meta.GetOptions) (*batchv1.Job, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
	}
	job, err := c.client.BatchV1().
		Jobs(getOptions.Namespace).
		Get(ctx, getOptions.ObjectName, metav1.GetOptions{})
	if err != nil {
		log.Logger.Errorf("failed to get %s statefulSets: %v", getOptions.Cloud, err)
		return nil, err
	}

	return job, err
}

func (c *jobs) List(ctx context.Context, listOptions meta.ListOptions) ([]batchv1.Job, error) {
	if c.client == nil {
		return nil, pixiuerrors.ErrCloudNotRegister
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
