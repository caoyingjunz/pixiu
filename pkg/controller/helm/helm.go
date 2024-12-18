package helm

import (
	"context"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type HelmGetter interface {
	Helm() Interface
}

type Interface interface {
	Release(cluster string) ReleaseInterface
	Repository() RepositoryInterface
}

type Helm struct {
	factory db.ShareDaoFactory
}

func (h *Helm) Release(cluster string) ReleaseInterface {
	cs, err := h.GetClusterSetByName(context.Background(), cluster)
	if err != nil {
		klog.Errorf("failed to get clusterSet: %v", err)
		return nil
	}
	settings := cli.New()
	actionConfig := new(action.Configuration)

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

func (h *Helm) GetClusterSetByName(ctx context.Context, name string) (client.ClusterSet, error) {
	cs, ok := cluster.ClusterIndexer.Get(name)
	if ok {
		klog.Infof("Get %s clusterSet from indexer", name)
		return cs, nil
	}

	klog.Infof("building clusterSet for %s", name)
	// 缓存中不存在，则新建并重写回缓存
	object, err := h.factory.Cluster().GetClusterByName(ctx, name)
	if err != nil {
		return client.ClusterSet{}, err
	}
	if object == nil {
		return client.ClusterSet{}, errors.ErrClusterNotFound
	}
	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return client.ClusterSet{}, err
	}

	klog.Infof("set %s clusterSet into indexer", name)
	cluster.ClusterIndexer.Set(name, *newClusterSet)
	return *newClusterSet, nil
}
