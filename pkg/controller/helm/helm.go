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

package helm

import (
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	helmClient "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/release"
)

type HelmGetter interface {
	Helm() Interface
}

type Interface interface {
	ListDeployedReleases(cloudName string, namespace string) ([]*release.Release, error)
}

type helm struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func NewHelm(cfg config.Config, f db.ShareDaoFactory) *helm {
	return &helm{
		cc:      cfg,
		factory: f,
	}
}

func (h helm) ListDeployedReleases(cloudName string, namespace string) ([]*release.Release, error) {
	helmClient, err := getHelmClient(cloudName, namespace)
	if err != nil {
		return nil, err
	}

	return helmClient.ListDeployedReleases()
}

func getHelmClient(cloudName string, namespace string) (helmClient.Client, error) {
	return nil, nil
}
