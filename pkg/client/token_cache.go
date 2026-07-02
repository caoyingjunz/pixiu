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

// TokenCache 实现登录状态缓存
type TokenCache struct {
	sync.RWMutex

	items map[int64]map[string]struct{}
}

func NewTokenCache() *TokenCache {
	return &TokenCache{
		items: map[int64]map[string]struct{}{},
	}
}

func (s *TokenCache) Get(uid int64) (string, bool) {
	s.RLock()
	defer s.RUnlock()

	tokens, ok := s.items[uid]
	if !ok || len(tokens) == 0 {
		return "", false
	}
	for token := range tokens {
		return token, true
	}
	return "", false
}

// Set 置空，只保留一个
func (s *TokenCache) Set(uid int64, token string) {
	s.Lock()
	defer s.Unlock()

	if s.items == nil {
		s.items = map[int64]map[string]struct{}{}
	}
	s.items[uid] = map[string]struct{}{token: {}}
}

// Add 可以存多个
func (s *TokenCache) Add(uid int64, token string) {
	s.Lock()
	defer s.Unlock()

	if s.items == nil {
		s.items = map[int64]map[string]struct{}{}
	}
	if s.items[uid] == nil {
		s.items[uid] = map[string]struct{}{}
	}
	s.items[uid][token] = struct{}{}
}

func (s *TokenCache) Delete(uid int64) {
	s.Lock()
	defer s.Unlock()

	delete(s.items, uid)
}

func (s *TokenCache) Clear() {
	s.Lock()
	defer s.Unlock()

	s.items = map[int64]map[string]struct{}{}
}

func (s *TokenCache) Exists(uid int64, token string) bool {
	s.RLock()
	defer s.RUnlock()

	tokens, ok := s.items[uid]
	if !ok {
		return false
	}
	_, exists := tokens[token]
	return exists
}

func (s *TokenCache) DeleteToken(uid int64, token string) {
	s.Lock()
	defer s.Unlock()

	tokens, ok := s.items[uid]
	if !ok {
		return
	}
	delete(tokens, token)
	if len(tokens) == 0 {
		delete(s.items, uid)
	}
}
