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

package core

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type CloudGetter interface {
	Cloud() CloudInterface
}

type CloudInterface interface {
	ListDeployments(ctx context.Context, namespace string) (*v1.DeploymentList, error)
	DeleteDeployment(ctx context.Context, namespace, name string) error
}

type cloud struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
	clientSet       *kubernetes.Clientset
}

func newCloud(c *pixiu) CloudInterface {
	return &cloud{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
		clientSet:       c.clientSet,
	}
}

func (c *cloud) ListDeployments(ctx context.Context, namespace string) (*v1.DeploymentList, error) {
	deployments, err := c.clientSet.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list deployments: %v", err)
		return nil, err
	}

	return deployments, nil
}

func (c *cloud) DeleteDeployment(ctx context.Context, namespace, name string) error {
	err := c.clientSet.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Logger.Errorf("failed to delete  %v deployments %v : %v", namespace, name, err)
		return err
	}
	return nil
}
