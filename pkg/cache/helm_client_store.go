package cache

import (
	"sync"

	client "github.com/mittwald/go-helm-client"
)

type clientStore map[string]client.Client

type HelmClientStore struct {
	sync.RWMutex
	clientStore
}

func (s *HelmClientStore) Get(cloudAndNamespace string) (client.Client, bool) {
	s.RLock()
	defer s.RUnlock()

	cluster, ok := s.clientStore[cloudAndNamespace]
	return cluster, ok
}

func (s *HelmClientStore) Set(cloudAndNamespace string, helmClient client.Client) {
	s.Lock()
	defer s.Unlock()

	if s.clientStore == nil {
		s.clientStore = clientStore{}
	}
	s.clientStore[cloudAndNamespace] = helmClient
}

func (s *HelmClientStore) Delete(clusterName string) {
	s.Lock()
	defer s.Unlock()

	delete(s.clientStore, clusterName)
}

func (s *HelmClientStore) List() clientStore {
	s.Lock()
	defer s.Unlock()

	return s.clientStore
}

func (s *HelmClientStore) Clear() {
	s.Lock()
	defer s.Unlock()

	s.clientStore = clientStore{}
}
