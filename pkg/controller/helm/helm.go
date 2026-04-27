/*
Copyright 2024 The Pixiu Authors.

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

package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db"
)

type HelmGetter interface {
	Helm() Interface
}

type Interface interface {
	Release(cluster, namespace string) ReleaseInterface
	Repository() RepositoryInterface
}

type Helm struct {
	factory db.ShareDaoFactory
}

func (h *Helm) Release(cluster, namespace string) ReleaseInterface {
	cs := h.MustGetClusterSetByName(context.Background(), cluster)
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	resetClientGetter := client.NewHelmRESTClientGetter(cs.Config)
	actionConfig.Init(
		resetClientGetter,
		settings.Namespace(),
		"secrets",
		klog.Infof,
	)
	return NewReleases(actionConfig, settings)
}

func (h *Helm) Repository() RepositoryInterface {
	return NewRepository(h.factory)
}

func NewHelm(factory db.ShareDaoFactory) Interface {
	return &Helm{
		factory: factory,
	}
}

func (h *Helm) MustGetClusterSetByName(ctx context.Context, name string) client.ClusterSet {
	cs, ok := cluster.ClusterIndexer.Get(name)
	if ok {
		klog.Infof("Get %s clusterSet from indexer", name)
		return cs
	}

	klog.Infof("building clusterSet for %s", name)
	// 缓存中不存在，则新建并重写回缓存
	object, err := h.factory.Cluster().GetClusterByName(ctx, name)
	if err != nil {
		return client.ClusterSet{}
	}
	if object == nil {
		return client.ClusterSet{}
	}
	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return client.ClusterSet{}
	}

	klog.Infof("set %s clusterSet into indexer", name)
	cluster.ClusterIndexer.Set(name, *newClusterSet)
	return *newClusterSet
}
