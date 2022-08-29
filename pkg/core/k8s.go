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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type K8sGetter interface {
	K8s() K8sInterface
}

type K8sInterface interface {
	ClusterCreate(ctx context.Context, obj *types.K8sClusterCreate) error
}

type k8s struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
	clientSets      map[string]*kubernetes.Clientset
}

func newK8s(c *pixiu) K8sInterface {
	return &k8s{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
		clientSets:      c.clientSets,
	}
}

func RegisterClientSets(factory db.ShareDaoFactory) (map[string]*kubernetes.Clientset, error) {
	clientSets := map[string]*kubernetes.Clientset{}
	clusters, err := factory.K8s().ClusterGetAll(context.TODO())
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

func (c *k8s) getClientSet(name string) (*kubernetes.Clientset, error) {
	clientSet := c.clientSets[name]
	if clientSet == nil {
		return nil, fmt.Errorf("cluster not found")
	}
	return clientSet, nil
}

func (c *k8s) ClusterCreate(ctx context.Context, obj *types.K8sClusterCreate) error {
	k8sCluster, err := c.factory.K8s().ClusterGetByName(ctx, obj.Name)
	if err != nil && !db.IsNotFound(err) {
		return err
	}
	if k8sCluster != nil {
		return fmt.Errorf("cluster already exists")
	}

	c.clientSets[obj.Name], err = newClientSet([]byte(obj.Config))
	if err != nil {
		log.Logger.Errorf("failed to create %s k8s cluster: %v", obj.Name, err)
		return err
	}

	_, err = c.factory.K8s().ClusterCreate(ctx, &model.K8sCluster{
		Name:   obj.Name,
		Config: obj.Config,
	})
	if err != nil {
		log.Logger.Errorf("failed to create %s k8s cluster: %v", obj.Name, err)
		return err
	}

	return nil
}
