package client

import "sync"

type RoleCache struct {
	sync.RWMutex

	items map[int64]map[string]bool
}

func NewRoleCache() *RoleCache {
	return &RoleCache{}
}

func (s *RoleCache) Valid(uid int64) bool {
	s.RLock()
	defer s.RUnlock()

	return true
}

func (s *RoleCache) Set(roleId int64, apis []string) {
	s.RLock()
	defer s.RUnlock()

}

func (s *RoleCache) Reset(role int64, apis []string) {
	s.RLock()
	defer s.RUnlock()

}
