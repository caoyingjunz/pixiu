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

package util

import (
	helmclient "github.com/mittwald/go-helm-client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/cache"
	"github.com/caoyingjunz/pixiu/pkg/util/uuid"
)

func NewCloudSet(configBytes []byte) (*cache.Cluster, error) {
	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	return &cache.Cluster{
		ClientSet:  clientSet,
		KubeConfig: kubeConfig,
	}, nil
}

func NewClientSet(data []byte) (*kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

func NewCloudName(prefix string) string {
	return prefix + uuid.NewUUID()[:8]
}

func NewHelmClient(namespace string, kubeConfig *rest.Config) (helmclient.Client, error) {
	opt := &helmclient.RestConfClientOptions{
		Options: &helmclient.Options{
			Namespace: namespace,
			Debug:     true,
			Linting:   false,
			DebugLog: func(format string, v ...interface{}) {
				klog.Infof(format, v)
			},
		},
		RestConfig: kubeConfig,
	}

	return helmclient.NewClientFromRestConf(opt)
}
