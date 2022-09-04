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
	"fmt"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type CloudGetter interface {
	Cloud() CloudInterface
}

type CloudInterface interface {
	Create(ctx context.Context, obj *types.Cloud) error
	Delete(ctx context.Context, cid int64) error

	InitCloudClients() error

	ListDeployments(ctx context.Context, clusterName string) (*v1.DeploymentList, error)
}

type cloud struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
	clientSets      ClientsInterface
}

func newCloud(c *pixiu) CloudInterface {
	return &cloud{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
		clientSets:      NewCloudClients(),
	}
}

func (c *cloud) InitCloudClients() error {
	cloudObjs, err := c.factory.Cloud().List(context.TODO())
	if err != nil {
		log.Logger.Errorf("failed to list exist clouds: %v", err)
		return err
	}
	for _, cloudObj := range cloudObjs {
		clientSet, err := c.newClientSet([]byte(cloudObj.KubeConfig))
		if err != nil {
			log.Logger.Errorf("failed to create %s clientSet: %v", cloudObj.Name, err)
			return err
		}
		c.clientSets.Add(cloudObj.Name, clientSet)
	}

	return nil
}

func (c *cloud) newClientSet(data []byte) (*kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func (c *cloud) preCreate(ctx context.Context, obj *types.Cloud) error {
	if len(obj.Name) == 0 {
		return fmt.Errorf("invalid empty cloud name")
	}
	if len(obj.KubeConfig) == 0 {
		return fmt.Errorf("invalid empty kubeconfig data")
	}

	return nil
}

func (c *cloud) Create(ctx context.Context, obj *types.Cloud) error {
	if err := c.preCreate(ctx, obj); err != nil {
		log.Logger.Errorf("failed to pre-check for %a created: %v", obj.Name, err)
		return err
	}

	// 先构造 clientSet，如果有异常，直接返回
	clientSet, err := c.newClientSet(obj.KubeConfig)
	if err != nil {
		log.Logger.Errorf("failed to create %s clientSet: %v", obj.Name, err)
		return err
	}
	if _, err = c.factory.Cloud().Create(ctx, &model.Cloud{
		Name:       obj.Name,
		KubeConfig: string(obj.KubeConfig),
	}); err != nil {
		log.Logger.Errorf("failed to create %s cloud: %v", obj.Name, err)
		return err
	}

	c.clientSets.Add(obj.Name, clientSet)
	return nil
}

func (c *cloud) Delete(ctx context.Context, cid int64) error {
	// TODO: 删除cloud的同时，直接返回，避免一次查询
	obj, err := c.factory.Cloud().Get(ctx, cid)
	if err != nil {
		log.Logger.Errorf("failed to get %s cloud: %v", cid, err)
		return err
	}
	if err = c.factory.Cloud().Delete(ctx, cid); err != nil {
		log.Logger.Errorf("failed to delete %s cloud: %v", cid, err)
		return err
	}

	c.clientSets.Delete(obj.Name)
	return nil
}

func (c *cloud) ListDeployments(ctx context.Context, cloud string) (*v1.DeploymentList, error) {
	clientSet, found := c.clientSets.Get(cloud)
	if !found {
		return nil, fmt.Errorf("failed to found %s client", cloud)
	}

	deployments, err := clientSet.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list deployments: %v", err)
		return nil, err
	}

	return deployments, nil
}
