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

// MySQLPing MySQL 连接探测结果
type MySQLPing struct {
	Connected bool   `json:"connected"`
	LatencyMs int64  `json:"latency_ms"`        // 探测往返耗时（毫秒）
	Message   string `json:"message,omitempty"` // 失败时的错误描述
	Address   string `json:"address,omitempty"` // 连接地址（脱敏，不含密码）
	Version   string `json:"version,omitempty"` // 探测成功时顺带返回版本
}

// MySQLServerInfo MySQL 实例概览（版本、运行状态、关键指标）
type MySQLServerInfo struct {
	Version          string `json:"version"`             // SELECT VERSION()
	UptimeInSeconds  int64  `json:"uptime_in_seconds"`   // SHOW STATUS Uptime
	ThreadsConnected int64  `json:"threads_connected"`   // 当前连接数
	MaxConnections   int64  `json:"max_connections"`     // 最大连接数
	QueriesPerSecond int64  `json:"queries_per_second"`  // Questions / Uptime 估算
	SlowQueries      int64  `json:"slow_queries"`        // 慢查询累计数
	QuestionCount    int64  `json:"question_count"`      // Questions 累计
	InnoDBRowsRead   int64  `json:"innodb_rows_read"`    // InnoDB 行读取累计
	BytesReceived    int64  `json:"bytes_received"`      // 累计接收字节
	BytesSent        int64  `json:"bytes_sent"`          // 累计发送字节
	ReadOnly         bool   `json:"read_only"`           // 实例是否只读
	DatabaseCount    int    `json:"database_count"`      // 用户库数量（排除系统库）
	DataSizeMB       int64  `json:"data_size_mb"`        // 全部库数据+索引体积（MB）
	SlowQueryLog     string `json:"slow_query_log"`      // slow_query_log 开关 ON/OFF
	LongQueryTime    string `json:"long_query_time"`     // 慢查询阈值（秒）
	SlowQueryLogFile string `json:"slow_query_log_file"` // 慢查询日志文件路径
}

// MySQLDatabase 数据库概览
type MySQLDatabase struct {
	Name      string `json:"name"`
	Charset   string `json:"charset"`   // 默认字符集
	Collation string `json:"collation"` // 默认排序规则
	SizeMB    int64  `json:"size_mb"`   // 数据+索引体积（MB）
	TableNum  int64  `json:"table_num"` // 表数量
}

// MySQLTable 表概览
type MySQLTable struct {
	Name       string  `json:"name"`
	Engine     string  `json:"engine"`
	Rows       int64   `json:"rows"`    // 估算行数
	SizeMB     float64 `json:"size_mb"` // 数据+索引体积（MB）
	CreateTime string  `json:"create_time,omitempty"`
	Comment    string  `json:"comment,omitempty"`
}

// MySQLColumn 列元数据
type MySQLColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`    // 列类型（含长度）
	Null       string `json:"null"`    // YES/NO
	Key        string `json:"key"`     // PRI/UNI/MUL/空
	Default    string `json:"default"` // 默认值（NULL 显示为空串）
	Extra      string `json:"extra"`   // auto_increment 等
	Comment    string `json:"comment"`
	OrdinalPos int64  `json:"ordinal_position"`
}

// MySQLTableDetail 表详情（DDL + 列 + 索引）
type MySQLTableDetail struct {
	Name    string        `json:"name"`
	DDL     string        `json:"ddl"` // SHOW CREATE TABLE
	Columns []MySQLColumn `json:"columns"`
	Indexes []MySQLIndex  `json:"indexes"`
}

// MySQLIndex 索引信息（聚合自 SHOW INDEX）
type MySQLIndex struct {
	Name      string `json:"name"`
	NonUnique bool   `json:"non_unique"`
	Columns   string `json:"columns"` // 逗号拼接的列名（按序）
	Type      string `json:"type"`    // BTREE/HASH 等
}

// MySQLQueryRequest SQL 控制台执行请求
type MySQLQueryRequest struct {
	Database string `json:"database"` // 执行库（可空表示实例级）
	SQL      string `json:"sql" binding:"required"`
	Limit    int64  `json:"limit,omitempty"` // SELECT 结果行数上限，缺省/超限时归一化
}

// MySQLQueryResult SQL 执行结果
type MySQLQueryResult struct {
	Columns   []string        `json:"columns,omitempty"`   // 结果列名（SELECT 类）
	Rows      [][]interface{} `json:"rows,omitempty"`      // 结果行
	Affected  int64           `json:"affected_rows"`       // 写操作影响行数
	Duration  int64           `json:"duration_ms"`         // 执行耗时（毫秒）
	Truncated bool            `json:"truncated,omitempty"` // 结果被截断时为 true
	Statement string          `json:"statement"`           // 语句类型：select/insert/update/delete/ddl/...
}

// MySQLExecuteBatchRequest SQL 控制台批量执行请求：原始文本一次提交，服务端拆分并逐条执行
type MySQLExecuteBatchRequest struct {
	Database string `json:"database"` // 执行库（可空表示实例级）
	SQL      string `json:"sql" binding:"required"`
	Limit    int64  `json:"limit,omitempty"` // 每条 SELECT 结果行数上限，缺省/超限时归一化
}

// MySQLExecuteBatchItem 单条语句的执行结果；ok=false 时 Error 携带错误信息
type MySQLExecuteBatchItem struct {
	Index     int               `json:"index"`               // 语句序号（从 1 开始）
	StartLine int               `json:"start_line"`          // 语句在原始文本中的起始行（从 1 开始），供前端定位
	Ok        bool              `json:"ok"`
	Error     string            `json:"error,omitempty"`
	Result    *MySQLQueryResult `json:"result,omitempty"`
}

// MySQLExecuteBatchResult 批量执行结果：遇错停止，StoppedAt 为出错语句序号（0 表示全部成功）
type MySQLExecuteBatchResult struct {
	Items     []MySQLExecuteBatchItem `json:"items"`
	StoppedAt int                     `json:"stopped_at"`
	Total     int                     `json:"total"` // 拆分出的语句总数
}

// MySQLUser 实例用户
type MySQLUser struct {
	User            string `json:"user"`
	Host            string `json:"host"`
	AccountLocked   bool   `json:"account_locked"`
	PasswordExpired bool   `json:"password_expired"`
	Privileges      string `json:"privileges,omitempty"` // SHOW GRANTS 摘要（按需填充）
}

// MySQLCreateUserRequest 创建用户请求
type MySQLCreateUserRequest struct {
	User     string `json:"user" binding:"required"`
	Host     string `json:"host"` // 缺省 %
	Password string `json:"password" binding:"required"`
	Grant    string `json:"grant,omitempty"` // 授权语句模板，如 "SELECT,INSERT ON db.*"；空表示只建用户
}

// MySQLGrantRequest 用户授权请求
type MySQLGrantRequest struct {
	User       string `json:"user" binding:"required"`
	Host       string `json:"host"`
	Privileges string `json:"privileges" binding:"required"` // SELECT, INSERT 或 ALL PRIVILEGES
	Object     string `json:"object" binding:"required"`     // 授权对象：*.* / db.* / db.table
}

// MySQLSession 实例会话（PROCESSLIST）
type MySQLSession struct {
	Id      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int64  `json:"time"`
	State   string `json:"state"`
	Info    string `json:"info"` // 正在执行的 SQL（截断保护）
}

// MySQLSlowQuery 慢查询记录
type MySQLSlowQuery struct {
	StartTime    string `json:"start_time"` // 由查询时间+耗时推算
	QueryTime    string `json:"query_time"` // 秒
	LockTime     string `json:"lock_time"`
	RowsSent     int64  `json:"rows_sent"`
	RowsExamined int64  `json:"rows_examined"`
	User         string `json:"user,omitempty"`
	Host         string `json:"host,omitempty"`
	DB           string `json:"db,omitempty"`
	SQLText      string `json:"sql_text"`
}

// MySQLSlowQueryList 慢查询列表及实例慢日志状态；
// log_output 不含 TABLE 时列表为空，前端依据状态展示原因与开启方式
type MySQLSlowQueryList struct {
	SlowQueryLog              string           `json:"slow_query_log"`
	LogOutput                 string           `json:"log_output"`
	SlowQueryLogFile          string           `json:"slow_query_log_file,omitempty"`
	LogQueriesNotUsingIndexes string           `json:"log_queries_not_using_indexes"`
	Total                     int64            `json:"total"` // slow_log 总记录数，供前端分页
	Items                     []MySQLSlowQuery `json:"items"`
}

// MySQLBackupRequest 备份请求（SQL 文本备份）
type MySQLBackupRequest struct {
	Database string   `json:"database" binding:"required"`
	Tables   []string `json:"tables,omitempty"`   // 空表示整库；指定时只备份列出的表
	WithData bool     `json:"with_data"`          // 是否包含数据（INSERT）
	MaxRows  int64    `json:"max_rows,omitempty"` // 单表最大备份行数，缺省/超限时归一化
}

// MySQLBackupResult 备份结果（SQL 文本）
type MySQLBackupResult struct {
	Database    string `json:"database"`
	GeneratedAt string `json:"generated_at"`
	TableNum    int    `json:"table_num"`
	Truncated   bool   `json:"truncated,omitempty"` // 存在表因行数上限被截断
	SizeBytes   int64  `json:"size_bytes"`
	Content     string `json:"content"` // SQL 文本
}
