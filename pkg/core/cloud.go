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
	"sync"

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
	ListDeployments(ctx context.Context, clusterName string) (*v1.DeploymentList, error)
	ClusterCreate(ctx context.Context, obj *types.CloudClusterCreate) error
	ClusterDelete(ctx context.Context, clusterName string) error
}

type cloud struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
	clientSets      map[string]*kubernetes.Clientset
}

func newCloud(c *pixiu) CloudInterface {
	return &cloud{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
		clientSets:      c.clientSets,
	}
}

func RegisterClientSets(factory db.ShareDaoFactory) (map[string]*kubernetes.Clientset, error) {
	clientSets := map[string]*kubernetes.Clientset{}
	clusters, err := factory.Cloud().ClusterGetAll(context.TODO())
	if err != nil {
		return nil, err
	}
	for _, v := range clusters {
		c, err := newClientSet([]byte(v.Config))
		if err != nil {
			return nil, err
		}
		clientSets[v.Name] = c
	}

	return clientSets, nil
}

func newClientSet(config []byte) (*kubernetes.Clientset, error) {
	c, err := clientcmd.RESTConfigFromKubeConfig(config)
	if err != nil {
		return nil, err
	}

	cs, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (c *cloud) getClientSet(name string) (*kubernetes.Clientset, error) {
	clientSet := c.clientSets[name]
	if clientSet == nil {
		return nil, fmt.Errorf("cluster not found")
	}
	return clientSet, nil
}

func (c *cloud) ClusterCreate(ctx context.Context, obj *types.CloudClusterCreate) error {
	var lock sync.RWMutex
	cloudCluster, err := c.factory.Cloud().ClusterGet(ctx, obj.Name)
	if err != nil && !db.IsNotFound(err) {
		return err
	}
	if cloudCluster != nil {
		return fmt.Errorf("cluster already exists")
	}

	lock.Lock()
	c.clientSets[obj.Name], err = newClientSet([]byte(obj.Config))
	lock.Unlock()
	if err != nil {
		log.Logger.Errorf("failed to create %s cloud cluster: %v", obj.Name, err)
		return err
	}

	_, err = c.factory.Cloud().ClusterCreate(ctx, &model.CloudCluster{
		Name:   obj.Name,
		Config: obj.Config,
	})
	if err != nil {
		log.Logger.Errorf("failed to create %s cloud cluster: %v", obj.Name, err)
		return err
	}

	return nil
}

func (c *cloud) ClusterDelete(ctx context.Context, clusterName string) error {
	if _, err := c.getClientSet(clusterName); err != nil {
		return err
	}
	delete(c.clientSets, clusterName)

	if _, err := c.factory.Cloud().ClusterDelete(ctx, clusterName); err != nil {
		log.Logger.Errorf("failed to delete %s cloud cluster: %v", clusterName, err)
		return err
	}

	return nil
}

func (c *cloud) ListDeployments(ctx context.Context, clusterName string) (*v1.DeploymentList, error) {
	clientSet, err := c.getClientSet(clusterName)
	if err != nil {
		return nil, err
	}
	deployments, err := clientSet.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Logger.Errorf("failed to list deployments: %v", err)
		return nil, err
	}

	return deployments, nil
}
