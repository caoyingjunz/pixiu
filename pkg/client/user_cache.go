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

type UserCache struct {
	sync.RWMutex
	items map[int64]int
}

func NewUserCache() *UserCache {
	return &UserCache{
		items: map[int64]int{},
	}
}

func (s *UserCache) Get(uid int64) (int, bool) {
	s.RLock()
	defer s.RUnlock()

	status, ok := s.items[uid]
	return status, ok
}

func (s *UserCache) Set(uid int64, status int) {
	s.RLock()
	defer s.RUnlock()

	if s.items == nil {
		s.items = map[int64]int{}
	}
	s.items[uid] = status
}

func (s *UserCache) Delete(uid int64) {
	s.RLock()
	defer s.RUnlock()

	delete(s.items, uid)
}

func (s *UserCache) Clear() {
	s.RLock()
	defer s.RUnlock()

	s.items = map[int64]int{}
}
