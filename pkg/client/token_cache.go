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

package client

import "sync"

// TokenCache
// TODO: 临时实现，后续优化
type TokenCache struct {
	sync.RWMutex
	items map[int64]string
}

func NewTokenCache() *TokenCache {
	return &TokenCache{
		items: map[int64]string{},
	}
}

func (s *TokenCache) Get(uid int64) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	t, ok := s.items[uid]
	return t, ok
}

func (s *TokenCache) Set(uid int64, token string) {
	s.RLock()
	defer s.RUnlock()

	if s.items == nil {
		s.items = map[int64]string{}
	}
	s.items[uid] = token
}

func (s *TokenCache) Delete(uid int64) {
	s.RLock()
	defer s.RUnlock()

	delete(s.items, uid)
}

func (s *TokenCache) Clear() {
	s.RLock()
	defer s.RUnlock()

	s.items = map[int64]string{}
}
