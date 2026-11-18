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

// RedisKeyspaceDB INFO keyspace 段中单个 DB 的 key 分布
type RedisKeyspaceDB struct {
	DB      string `json:"db"`      // e.g. "db0"
	Keys    int64  `json:"keys"`    // key 数量
	Expires int64  `json:"expires"` // 设置了过期时间的 key 数量
	AvgTTL  int64  `json:"avg_ttl"` // 平均 TTL（毫秒）
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
	TotalKeys        int64  `json:"total_keys"`

	UsedMemory             int64   `json:"used_memory"`
	UsedMemoryRss          int64   `json:"used_memory_rss"`
	UsedMemoryPeak         int64   `json:"used_memory_peak"`
	UsedMemoryLua          int64   `json:"used_memory_lua"`
	MaxMemory              int64   `json:"max_memory"`
	MaxMemoryHuman         string  `json:"max_memory_human"`
	MaxMemoryPolicy        string  `json:"max_memory_policy"`
	TotalSystemMemory      int64   `json:"total_system_memory"`
	TotalSystemMemoryHuman string  `json:"total_system_memory_human"`
	MemFragmentation       float64 `json:"mem_fragmentation_ratio"`

	TotalCommands    int64 `json:"total_commands_processed"`
	InstantaneousOps int64 `json:"instantaneous_ops_per_sec"`
	EvictedKeys      int64 `json:"evicted_keys"`
	ExpiredKeys      int64 `json:"expired_keys"`

	// 连接
	BlockedClients int64 `json:"blocked_clients"`
	RejectedConns  int64 `json:"rejected_connections"`

	// 持久化
	RdbLastSaveStatus string `json:"rdb_last_bgsave_status"`
	RdbLastSaveTime   int64  `json:"rdb_last_save_time"`
	AofEnabled        int64  `json:"aof_enabled"`

	// 复制
	Role            string `json:"role"` // master/slave
	ConnectedSlaves int64  `json:"connected_slaves"`

	// Keyspace 各 DB 分布
	KeyspaceDBs []RedisKeyspaceDB `json:"keyspace_dbs"`
}

// RedisKeyItem SCAN 扫描出的 key 概览（列表仅返回元数据，不含 value）
type RedisKeyItem struct {
	Key  string `json:"key"`
	Type string `json:"type"` // string/hash/list/set/zset/stream 等
	TTL  int64  `json:"ttl"`  // 秒；-1 永不过期，-2 key 已不存在
}

// RedisScanResult SCAN 分页结果（cursor 透传，后端无状态）
type RedisScanResult struct {
	Cursor uint64         `json:"cursor"` // 下一次 SCAN 起始游标；0 表示遍历结束
	Keys   []RedisKeyItem `json:"keys"`
}

// RedisKeyDetail 单个 key 的详情（只读）
type RedisKeyDetail struct {
	Key       string      `json:"key"`
	Type      string      `json:"type"`
	TTL       int64       `json:"ttl"`
	Encoding  string      `json:"encoding,omitempty"`
	SizeBytes int64       `json:"size_bytes"`          // MEMORY USAGE 估算
	Value     interface{} `json:"value"`               // string 为字符串（GETRANGE 拉取，超限截断）；hash 为 map；list/set/zset 为数组
	Truncated bool        `json:"truncated,omitempty"` // 值过大被截断时为 true；截断时禁止经控制台编辑保存
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

// RedisDeleteKeysRequest 批量删除 key 请求
type RedisDeleteKeysRequest struct {
	Keys []string `json:"keys" binding:"required"`
	DB   *int     `json:"db,omitempty"`
}

// RedisDeleteKeysResult 批量删除 key 结果
type RedisDeleteKeysResult struct {
	Deleted int64 `json:"deleted"` // 实际删除的 key 数量
}

// RedisUpdateKeyValueRequest 修改 string 类型 key 的值请求（保持原 TTL）
type RedisUpdateKeyValueRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
	DB    *int   `json:"db,omitempty"`
}

// RedisConfigItem Redis CONFIG GET 返回的单个参数
type RedisConfigItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RedisSetConfigRequest 修改运行期配置（CONFIG SET）
type RedisSetConfigRequest struct {
	Name  string `json:"name" binding:"required"`
	Value string `json:"value"`
}

// RedisACLUser Redis ACL 用户（解析自 ACL LIST）
type RedisACLUser struct {
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	NoPass   bool     `json:"nopass"`
	Commands string   `json:"commands"`
	Keys     []string `json:"keys"`
	Channels []string `json:"channels"`
	Flags    []string `json:"flags"`
}

// RedisACLListResult ACL 用户列表；Supported=false 表示实例不支持或无权执行 ACL
type RedisACLListResult struct {
	Supported bool           `json:"supported"`
	Message   string         `json:"message,omitempty"`
	Users     []RedisACLUser `json:"users"`
}

// RedisCreateACLUserRequest 创建 ACL 用户
type RedisCreateACLUserRequest struct {
	Name     string   `json:"name" binding:"required"`
	Password string   `json:"password"`
	NoPass   bool     `json:"nopass"`
	Enabled  *bool    `json:"enabled"`
	Keys     string   `json:"keys"`
	Commands []string `json:"commands"`
}

// RedisGrantACLUserRequest 为 ACL 用户追加命令分类或 Key 模式
type RedisGrantACLUserRequest struct {
	Name     string   `json:"name" binding:"required"`
	Commands []string `json:"commands"`
	Keys     string   `json:"keys"`
}

// RedisSlowLogItem SLOWLOG GET 中的一条记录
type RedisSlowLogItem struct {
	ID         int64    `json:"id"`
	Timestamp  int64    `json:"timestamp"`
	DurationUs int64    `json:"duration_us"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	ClientAddr string   `json:"client_addr"`
	ClientName string   `json:"client_name"`
}

// RedisClientItem CLIENT LIST 中的一个连接
type RedisClientItem struct {
	ID     string `json:"id"`
	Addr   string `json:"addr"`
	Name   string `json:"name"`
	Age    int64  `json:"age"`
	Idle   int64  `json:"idle"`
	Flags  string `json:"flags"`
	DB     string `json:"db"`
	Cmd    string `json:"cmd"`
	User   string `json:"user"`
	TotMem int64  `json:"tot_mem"`
}
