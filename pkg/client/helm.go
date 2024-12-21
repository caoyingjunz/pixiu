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

package client

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type HelmRESTClientGetter struct {
	kubeConfig *rest.Config
}

var _ genericclioptions.RESTClientGetter = &HelmRESTClientGetter{}

// ToDiscoveryClient implements action.RESTClientGetter.
func (h *HelmRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	h.kubeConfig.Burst = 100
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(h.kubeConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(discoveryClient), nil
}

// ToRESTConfig implements action.RESTClientGetter.
func (h *HelmRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return h.kubeConfig, nil
}

// ToRESTMapper implements action.RESTClientGetter.
func (h *HelmRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {

	discoveryClient, err := h.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (h *HelmRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

func NewHelmRESTClientGetter(kubeConfig *rest.Config) *HelmRESTClientGetter {
	return &HelmRESTClientGetter{
		kubeConfig: kubeConfig,
	}
}
