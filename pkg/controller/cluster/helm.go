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

package cluster

import (
	"context"

	helmclient "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/klog/v2"
)

func (c *cluster) ListReleases(ctx context.Context, cluster string, namespace string) ([]*release.Release, error) {
	helmClient, err := c.buildHelmClient(ctx, cluster, namespace)
	if err != nil {
		klog.Errorf("failed to build helm client: %v", err)
		return nil, err
	}

	releases, err := helmClient.ListDeployedReleases()
	if err != nil {
		klog.Errorf("failed to list helm release: %v", err)
		return nil, err
	}
	return releases, nil
}

func (c *cluster) buildHelmClient(ctx context.Context, cluster string, namespace string) (helmclient.Client, error) {
	kubeConfig, err := c.GetKubeConfigByName(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// TODO: 目前 helm 的官方库，不支持在实例化之后修改 namespace，只能重新构造
	return helmclient.NewClientFromRestConf(&helmclient.RestConfClientOptions{
		Options: &helmclient.Options{
			Namespace: namespace,
			Debug:     true,
			Linting:   false,
			DebugLog: func(format string, v ...interface{}) {
				klog.Infof(format, v)
			},
		},
		RestConfig: kubeConfig,
	})
}
