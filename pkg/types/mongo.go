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

// MongoPing MongoDB 连接探测结果
type MongoPing struct {
	Connected bool   `json:"connected"`
	LatencyMs int64  `json:"latency_ms"`        // 探测往返耗时（毫秒）
	Message   string `json:"message,omitempty"` // 失败时的错误描述
	Address   string `json:"address,omitempty"` // 连接地址（脱敏，不含密码）
	Version   string `json:"version,omitempty"` // 探测成功时顺带返回版本
}

// MongoInfo MongoDB 实例概览（解析自 dbStats / serverStatus）
type MongoInfo struct {
	Version        string `json:"version"`               // MongoDB 版本
	UptimeSeconds  int64  `json:"uptime_seconds"`        // 运行时长（秒）
	CurrentOp      int64  `json:"current_op"`            // 当前操作数
	Connections    int64  `json:"connections"`           // 当前连接数
	AvailableConns int64  `json:"available_conns"`       // 可用连接数
	TotalCreated   int64  `json:"total_created"`         // 累计创建连接数
	DataSize       int64  `json:"data_size"`             // 数据大小（字节，dbStats.dataSize）
	StorageSize    int64  `json:"storage_size"`          // 存储大小（字节）
	IndexSize      int64  `json:"index_size"`            // 索引大小（字节）
	Collections    int64  `json:"collections"`           // 集合数量
	Objects        int64  `json:"objects"`               // 文档总数
	DbName         string `json:"db_name"`               // 统计的数据库名
	Host           string `json:"host,omitempty"`        // 主机信息
	ReplicaSet     string `json:"replica_set,omitempty"` // 副本集名称
	Role           string `json:"role,omitempty"`        // 节点角色（PRIMARY/SECONDARY）

	// 操作计数（opcounters）
	Insert  int64 `json:"insert"`
	Query   int64 `json:"query"`
	Update  int64 `json:"update"`
	Delete  int64 `json:"delete"`
	Command int64 `json:"command"`
	GetMore int64 `json:"get_more"`

	// 网络流量（bytes in/out）
	BytesIn  int64 `json:"bytes_in"`
	BytesOut int64 `json:"bytes_out"`
}

// MongoReplSetMember 副本集成员状态
type MongoReplSetMember struct {
	Name       string `json:"name"`        // 成员地址 host:port
	StateStr   string `json:"state_str"`   // PRIMARY/SECONDARY/ARBITER/DOWN
	Health     int    `json:"health"`      // 健康状态 1=正常 0=异常
	UptimeSec  int64  `json:"uptime_sec"`  // 成员运行时长（秒）
	PingMs     int64  `json:"ping_ms"`     // 心跳延迟
	OptimeDate string `json:"optime_date"` // 最新操作时间
	Self       bool   `json:"self"`        // 是否当前连接节点
}

// MongoReplSetResult 副本集状态结果
type MongoReplSetResult struct {
	Supported bool                 `json:"supported"`         // 是否为副本集（单机为 false）
	SetName   string               `json:"set_name"`          // 副本集名称
	MyState   string               `json:"my_state"`          // 当前节点角色
	Members   []MongoReplSetMember `json:"members"`           // 成员列表
	Message   string               `json:"message,omitempty"` // 不支持或失败时的说明
}

// MongoSlowLogItem 慢日志条目（来自 system.profile）
type MongoSlowLogItem struct {
	Timestamp    string `json:"timestamp"`              // 时间戳
	DurationMs   int64  `json:"duration_ms"`            // 耗时（毫秒）
	Op           string `json:"op"`                     // 操作类型 query/update/remove/getmore/command
	Namespace    string `json:"namespace"`              // 库.集合
	Command      string `json:"command"`                // 命令摘要
	PlanSummary  string `json:"plan_summary,omitempty"` // 执行计划摘要
	DocsExamined int64  `json:"docs_examined"`          // 扫描文档数
	NReturned    int64  `json:"n_returned"`             // 返回文档数
	Client       string `json:"client,omitempty"`       // 客户端地址
}

// MongoSlowLogResult 慢日志查询结果
type MongoSlowLogResult struct {
	Supported bool               `json:"supported"` // 是否开启 profiling
	Items     []MongoSlowLogItem `json:"items"`
	Message   string             `json:"message,omitempty"`
}

// MongoParamItem 参数列表条目
type MongoParamItem struct {
	Name  string `json:"name"`  // 参数名
	Value string `json:"value"` // 参数值（已转为字符串）
}

// MongoParamResult 参数列表结果
type MongoParamResult struct {
	Supported bool             `json:"supported"`
	Items     []MongoParamItem `json:"items"`
	Message   string           `json:"message,omitempty"`
}
