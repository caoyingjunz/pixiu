/*
Copyright 2026 The Pixiu Authors.

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

package storage

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/controller/extension/storage/adapter"
)

const (
	// defaultClientCacheMaxEntries 客户端缓存的 LRU 容量上限。
	defaultClientCacheMaxEntries = 128
	// defaultClientCacheTTL 客户端缓存的兜底有效期；配置变更主要由 fingerprint 立即失效。
	defaultClientCacheTTL = 5 * time.Minute
	// defaultClientCacheSweepInterval 主动清扫过期条目的最小间隔。
	defaultClientCacheSweepInterval = time.Minute
)

// cachedClient 是单个数据源已构造好的适配器实例。
type cachedClient struct {
	datasourceID int64
	fingerprint  [32]byte
	client       adapter.Adapter
	expiresAt    time.Time
	element      *list.Element
}

// clientCache 按数据源 ID 缓存对象存储客户端（LRU + TTL）。
// 适配器实现只持有可并发共享的 SDK 客户端，因此缓存条目可被多个请求同时使用。
type clientCache struct {
	mu            sync.Mutex
	max           int
	ttl           time.Duration
	sweepInterval time.Duration
	lastSweep     time.Time
	entries       map[int64]*cachedClient
	order         *list.List
	now           func() time.Time
}

func newClientCache(max int, ttl time.Duration) *clientCache {
	return &clientCache{
		max:           max,
		ttl:           ttl,
		sweepInterval: defaultClientCacheSweepInterval,
		entries:       make(map[int64]*cachedClient),
		order:         list.New(),
		now:           time.Now,
	}
}

// sweepLocked 最多每 sweepInterval 执行一次，清理已过期条目；
// 覆盖「数据源被删除或不再访问，但条目仍占用 map」的场景，避免失效凭据长期常驻内存。
func (c *clientCache) sweepLocked(now time.Time) {
	if c.sweepInterval > 0 && now.Sub(c.lastSweep) < c.sweepInterval {
		return
	}
	c.lastSweep = now
	// Go 允许遍历 map 时删除当前元素。
	for _, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			c.removeLocked(entry)
		}
	}
}

// get 返回未过期且指纹匹配的客户端；未命中、过期或配置指纹变化时删除条目并返回 false。
func (c *clientCache) get(datasourceID int64, fingerprint [32]byte) (adapter.Adapter, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.sweepLocked(now)
	entry, ok := c.entries[datasourceID]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expiresAt) || entry.fingerprint != fingerprint {
		c.removeLocked(entry)
		return nil, false
	}
	c.order.MoveToFront(entry.element)
	return entry.client, true
}

// put 写入或替换数据源对应的客户端，并按 LRU 淘汰超出容量的条目。
func (c *clientCache) put(datasourceID int64, fingerprint [32]byte, client adapter.Adapter) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.sweepLocked(now)
	if entry, ok := c.entries[datasourceID]; ok {
		c.removeLocked(entry)
	}
	entry := &cachedClient{
		datasourceID: datasourceID,
		fingerprint:  fingerprint,
		client:       client,
		expiresAt:    now.Add(c.ttl),
	}
	entry.element = c.order.PushFront(entry)
	c.entries[datasourceID] = entry
	for c.order.Len() > c.max {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		c.removeLocked(oldest.Value.(*cachedClient))
	}
}

func (c *clientCache) removeLocked(entry *cachedClient) {
	delete(c.entries, entry.datasourceID)
	if entry.element != nil {
		c.order.Remove(entry.element)
		entry.element = nil
	}
}

// adapterConfigFingerprint 计算影响客户端构造的配置指纹，用于在配置变更后使缓存立即失效。
// 所有字段按长度前缀序列化，避免简单拼接产生歧义；只返回 SHA-256 摘要，不记录也不打印任何密钥。
// 注意：新增会影响客户端构造的 adapter.Config 字段时，必须同步加入下面的序列化（当前版本标记 v1）。
func adapterConfigFingerprint(cfg adapter.Config, providerKey string) [32]byte {
	var buf bytes.Buffer
	writeString := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		buf.Write(size[:])
		buf.WriteString(value)
	}

	writeString("v1")
	writeString(providerKey)
	writeString(cfg.Endpoint)
	if cfg.Secure {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	writeString(cfg.Region)
	writeString(cfg.AccessKeyID)
	writeString(cfg.SecretAccessKey)
	writeString(cfg.SessionToken)
	writeString(cfg.SignatureVersion)
	var bucketLookup [8]byte
	binary.BigEndian.PutUint64(bucketLookup[:], uint64(int64(cfg.BucketLookup)))
	buf.Write(bucketLookup[:])

	return sha256.Sum256(buf.Bytes())
}
