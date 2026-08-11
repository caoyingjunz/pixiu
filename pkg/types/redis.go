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

package types

// RedisPing Redis 连接探测结果
type RedisPing struct {
	Connected bool   `json:"connected"`
	LatencyMs int64  `json:"latency_ms"`        // 探测往返耗时（毫秒）
	Message   string `json:"message,omitempty"` // 失败时的错误描述
	Address   string `json:"address,omitempty"` // 连接地址（脱敏，不含密码）
	Version   string `json:"version,omitempty"` // 探测成功时顺带返回版本
	DB        int    `json:"db,omitempty"`
}

// RedisInfo Redis 实例概览（解析自 INFO 命令）
type RedisInfo struct {
	RedisVersion     string `json:"redis_version"`
	RedisMode        string `json:"redis_mode"`
	OS               string `json:"os"`
	UptimeInSeconds  int64  `json:"uptime_in_seconds"`
	UsedMemoryHuman  string `json:"used_memory_human"`
	ConnectedClients int64  `json:"connected_clients"`
	KeyspaceHits     int64  `json:"keyspace_hits"`
	KeyspaceMisses   int64  `json:"keyspace_misses"`
	TotalKeys        int64  `json:"total_keys"` // DBSIZE
	Raw              string `json:"raw"`        // INFO 命令原始输出
}

// RedisKeyItem SCAN 扫描出的 key 概览
type RedisKeyItem struct {
	Key  string `json:"key"`
	Type string `json:"type"` // string/hash/list/set/zset/stream 等
	TTL  int64  `json:"ttl"`  // 秒；-1 永不过期，-2 key 已不存在
}

// RedisScanResult SCAN 分页结果
type RedisScanResult struct {
	Page     int64          `json:"page"`
	PageSize int64          `json:"page_size"`
	HasMore  bool           `json:"has_more"` // 是否还有下一页
	Keys     []RedisKeyItem `json:"keys"`
}

// RedisKeyDetail 单个 key 的详情（只读）
type RedisKeyDetail struct {
	Key       string      `json:"key"`
	Type      string      `json:"type"`
	TTL       int64       `json:"ttl"`
	Encoding  string      `json:"encoding,omitempty"`
	SizeBytes int64       `json:"size_bytes"`          // MEMORY USAGE 估算
	Value     interface{} `json:"value"`               // string 为字符串；hash 为 map；list/set/zset 为数组
	Truncated bool        `json:"truncated,omitempty"` // 值过大被截断时为 true
}

// RedisCreateKeyRequest 新增 key 请求（第一版仅支持 string 类型）
type RedisCreateKeyRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl,omitempty"` // 过期秒数；<=0 表示永不过期
	DB    *int   `json:"db,omitempty"`  // 逻辑库编号；缺省用数据源配置默认
}

// RedisSetTTLRequest 修改 key TTL 请求
type RedisSetTTLRequest struct {
	Key string `json:"key" binding:"required"`
	TTL int64  `json:"ttl"` // 秒；>=0 设置过期，-1 移除过期（PERSIST）
	DB  *int   `json:"db,omitempty"`
}
