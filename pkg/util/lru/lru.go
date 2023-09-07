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

package lru

import (
	"container/list"
	"sync"
)

type LRUCache struct {
	cap       int
	evictList *list.List
	items     map[interface{}]*list.Element

	mu sync.RWMutex
}

type entry struct {
	key   interface{}
	value interface{}
}

func NewLRUCache(cap int) *LRUCache {
	return &LRUCache{
		cap:       cap,
		evictList: list.New(),
		items:     make(map[interface{}]*list.Element),
	}
}

func (c *LRUCache) Contains(key interface{}) bool {
	_, exists := c.items[key]
	return exists
}

func (c *LRUCache) Add(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: 重复代码优化
	if ent, ok := c.items[key]; ok {
		// 当前元素存在, 覆盖当前 value, 并移动到 list 头部
		ent.Value.(*entry).value = value
		c.evictList.MoveToFront(ent)
	} else {
		// 当前元素不存在, 并移动到 list 头部
		c.items[key] = c.evictList.PushFront(&entry{key, value})
	}

	// 超出 LRUCache 的容量, 删除 list 尾部元素
	if c.evictList.Len() > c.cap {
		lastElement := c.evictList.Back()
		c.evictList.Remove(lastElement)
		delete(c.items, lastElement.Value.(*entry).key)
	}
}

func (c *LRUCache) Get(key interface{}) (value interface{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if ent, ok := c.items[key]; ok {
		value = ent.Value.(*entry).value
		c.evictList.MoveToFront(ent)
	}
	return
}

func (c *LRUCache) Len() int { return c.evictList.Len() }
