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

package cache

import (
	"sync"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

type Cluster struct {
	ClientSet  *kubernetes.Clientset
	KubeConfig *restclient.Config
}

type store map[string]Cluster

type ClustersStore struct {
	sync.RWMutex
	store
}

func (s *ClustersStore) Get(clusterName string) (Cluster, bool) {
	s.RLock()
	defer s.RUnlock()

	cluster, ok := s.store[clusterName]
	return cluster, ok
}

func (s *ClustersStore) Set(clusterName string, cluster Cluster) {
	s.Lock()
	defer s.Unlock()

	if s.store == nil {
		s.store = store{}
	}
	s.store[clusterName] = cluster
}

func (s *ClustersStore) Delete(clusterName string) {
	s.Lock()
	defer s.Unlock()

	delete(s.store, clusterName)
}

func (s *ClustersStore) List() store {
	s.Lock()
	defer s.Unlock()

	return s.store
}

func (s *ClustersStore) Clear() {
	s.Lock()
	defer s.Unlock()

	s.store = store{}
}
