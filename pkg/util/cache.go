package util

import (
	"container/list"
	"fmt"
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

func NewLRUCache(cap int) (*LRUCache, error) {
	if cap <= 0 {
		return nil, fmt.Errorf("must provide a positive capacity")
	}
	return &LRUCache{
		cap:       cap,
		evictList: list.New(),
		items:     make(map[interface{}]*list.Element),
	}, nil
}

func (c *LRUCache) Contains(key interface{}) bool {
	_, exists := c.items[key]
	return exists
}

func (c *LRUCache) Add(key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, ok := c.items[key]; ok { // 当前 key 是否已经存在, 存在覆盖当前 value, 并把节点移动到 list 头部
		ent.Value.(*entry).value = value
		c.evictList.MoveToFront(ent)
	} else { // 不存在, 添加到 list 头部
		ent := &entry{key, value}
		c.items[key] = c.evictList.PushFront(ent)
	}

	// 已经超出 lrucache 的容量, 删除 list 尾部节点, 删除 items 中 key=key 的项
	if c.evictList.Len() >= c.cap {
		lastElement := c.evictList.Back()
		c.evictList.Remove(lastElement)
		delete(c.items, lastElement.Value.(*entry).key)
	}
}

func (c *LRUCache) Get(key interface{}) (value interface{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value = nil
	if ent, ok := c.items[key]; ok {
		value = ent.Value.(*entry).value
		c.evictList.MoveToFront(ent)
	}
	return
}

func (c *LRUCache) Len() int {
	return c.evictList.Len()
}
