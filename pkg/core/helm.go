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
	"fmt"

	client "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/release"

	"github.com/caoyingjunz/pixiu/pkg/log"
)

type HelmGetter interface {
	Helm() HelmInterface
}

type HelmInterface interface {
	ListDeployedReleases(cloudName string, namespace string) ([]*release.Release, error)
}

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
