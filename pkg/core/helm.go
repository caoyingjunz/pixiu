package core

import (
	"fmt"

	client "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/release"

	"github.com/caoyingjunz/pixiu/pkg/cache"
	"github.com/caoyingjunz/pixiu/pkg/log"
)

type HelmGetter interface {
	Helm() HelmInterface
}

type HelmInterface interface {
	ListDeployedReleases(cloudName string, namespace string) ([]*release.Release, error)
}

var (
	helmClientSet cache.HelmClientStore
)

type helm struct {
	app *pixiu
}

func newHelm(c *pixiu) HelmInterface {
	return &helm{
		app: c,
	}
}

func (h helm) ListDeployedReleases(cloudName string, namespace string) ([]*release.Release, error) {
	helmClient, err := getHelmClient(cloudName, namespace)
	if err != nil {
		return nil, err
	}
	releases, err := helmClient.ListDeployedReleases()
	return releases, err
}

func getHelmClient(cloudName string, namespace string) (client.Client, error) {
	var (
		client    client.Client
		present   bool
		err       error
		cachedKey = fmt.Sprintf("%s-%s", cloudName, namespace)
	)
	if client, present = helmClientSet.Get(cachedKey); !present {
		//create helmClient and cached
		if client, err = createHelmClient(cloudName, namespace); err != nil {
			return client, err
		}
		helmClientSet.Set(cachedKey, client)
	}
	return client, nil
}

func createHelmClient(cloudName string, namespace string) (client.Client, error) {
	cluster, exists := clusterSets.Get(cloudName)
	if !exists {
		return nil, fmt.Errorf("cluster %q not register", cloudName)
	}
	opt := &client.RestConfClientOptions{
		Options: &client.Options{
			Namespace:        namespace,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			Linting:          false,
			DebugLog: func(format string, v ...interface{}) {
				log.Logger.Infof(format, v)
			},
		},
		RestConfig: cluster.KubeConfig,
	}
	client, err := client.NewClientFromRestConf(opt)
	return client, err
}
