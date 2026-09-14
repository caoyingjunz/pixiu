package types

type PostgresPing struct {
	Connected bool   `json:"connected"`
	LatencyMs int64  `json:"latencyMs"`
	Message   string `json:"message,omitempty"`
	Address   string `json:"address,omitempty"`
	Version   string `json:"version,omitempty"`
}
type PostgresServerInfo struct {
	Version             string  `json:"version"`
	UptimeSeconds       int64   `json:"uptimeSeconds"`
	Connections         int64   `json:"connections"`
	MaxConnections      int64   `json:"maxConnections"`
	TPS                 float64 `json:"tps"`
	TotalTransactions   int64   `json:"totalTransactions"`
	DatabaseCount       int     `json:"databaseCount"`
	UserDatabaseCount   int     `json:"userDatabaseCount"`
	DataSizeBytes       int64   `json:"dataSizeBytes"`
	CacheHitRatio       float64 `json:"cacheHitRatio"`
	ActiveSessions      int64   `json:"activeSessions"`
	IdleSessions        int64   `json:"idleSessions"`
	IdleInTxSessions    int64   `json:"idleInTxSessions"`
	WaitingSessions     int64   `json:"waitingSessions"`
	BlockingSessions    int64   `json:"blockingSessions"`
	BlockedSessions     int64   `json:"blockedSessions"`
	Deadlocks           int64   `json:"deadlocks"`
	TempFiles           int64   `json:"tempFiles"`
	TempBytes           int64   `json:"tempBytes"`
	SharedBuffersBytes  int64   `json:"sharedBuffersBytes"`
	EffectiveCacheBytes int64   `json:"effectiveCacheBytes"`
	SlowQueries         int64   `json:"slowQueries"`
	PgStatStatements    bool    `json:"pgStatStatements"`
	InRecovery          bool    `json:"inRecovery"`
}
type PostgresDatabase struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	SizeBytes int64  `json:"sizeBytes"`
	Encoding  string `json:"encoding"`
	Collation string `json:"collation"`
}
type PostgresSchema struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}
type PostgresTable struct {
	Schema    string `json:"schema"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Rows      int64  `json:"rows"`
	SizeBytes int64  `json:"sizeBytes"`
}
type PostgresQueryRequest struct {
	Database string `json:"database"` // 执行库（PG 通过 SET search_path 切换）
	Schema   string `json:"schema"`
	SQL      string `json:"sql" binding:"required"`
	Limit    int64  `json:"limit,omitempty"`
}
type PostgresQueryResult struct {
	Columns   []string        `json:"columns,omitempty"`
	Rows      [][]interface{} `json:"rows,omitempty"`
	Affected  int64           `json:"affectedRows"`
	Duration  int64           `json:"durationMs"`
	Truncated bool            `json:"truncated,omitempty"`
	Statement string          `json:"statement"`
}
type PostgresSession struct {
	PID             int64   `json:"pid"`
	User            string  `json:"user"`
	ClientAddr      string  `json:"clientAddr"`
	Database        string  `json:"database"`
	Application     string  `json:"application"`
	State           string  `json:"state"`
	WaitEvent       string  `json:"waitEvent"`
	QueryStart      string  `json:"queryStart"`
	DurationSeconds float64 `json:"durationSeconds"`
	Query           string  `json:"query"`
}

// ── 批量执行 ──────────────────────────────────────────────────

// PostgresBatchRequest SQL 控制台批量执行请求
type PostgresBatchRequest struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	SQL      string `json:"sql" binding:"required"`
	Limit    int64  `json:"limit,omitempty"`
}

// PostgresBatchItem 单条语句的执行结果
type PostgresBatchItem struct {
	Index     int                  `json:"index"`
	StartLine int                  `json:"startLine"`
	Ok        bool                 `json:"ok"`
	Error     string               `json:"error,omitempty"`
	Result    *PostgresQueryResult `json:"result,omitempty"`
}

// PostgresBatchResult 批量执行结果：遇错停止
type PostgresBatchResult struct {
	Items     []PostgresBatchItem `json:"items"`
	StoppedAt int                 `json:"stoppedAt"`
	Total     int                 `json:"total"`
}

// ── 表详情 ────────────────────────────────────────────────────

// PostgresTableDetail 表详情（DDL + 列 + 索引 + 外键）
type PostgresTableDetail struct {
	Name        string               `json:"name"`
	Schema      string               `json:"schema"`
	DDL         string               `json:"ddl"`
	Rows        int64                `json:"rows"`
	SizeBytes   int64                `json:"sizeBytes"`
	RelKind     string               `json:"relKind,omitempty"`
	Columns     []PostgresColumn     `json:"columns"`
	Indexes     []PostgresIndex      `json:"indexes"`
	ForeignKeys []PostgresForeignKey `json:"foreignKeys"`
}

// PostgresColumn 列元数据
type PostgresColumn struct {
	Name         string `json:"name"`
	DataType     string `json:"dataType"`
	FullType     string `json:"fullType"`
	Nullable     bool   `json:"nullable"`
	Default      string `json:"default"`
	IsPrimaryKey bool   `json:"isPrimaryKey"`
	Comment      string `json:"comment"`
	OrdinalPos   int    `json:"ordinalPosition"`
}

// PostgresIndex 索引信息
type PostgresIndex struct {
	Name      string `json:"name"`
	IsUnique  bool   `json:"isUnique"`
	IsPrimary bool   `json:"isPrimary"`
	Columns   string `json:"columns"`
	Type      string `json:"type"`
	Def       string `json:"def"`
}

// PostgresForeignKey 外键信息
type PostgresForeignKey struct {
	Name       string `json:"name"`
	Columns    string `json:"columns"`
	RefSchema  string `json:"refSchema"`
	RefTable   string `json:"refTable"`
	RefColumns string `json:"refColumns"`
	OnUpdate   string `json:"onUpdate"`
	OnDelete   string `json:"onDelete"`
}

// PostgresCreateTableRequest 建表请求
type PostgresCreateTableRequest struct {
	Database string `json:"database" binding:"required"`
	Schema   string `json:"schema"`
	SQL      string `json:"sql" binding:"required"`
}

// PostgresAlterTableRequest 编辑表请求
type PostgresAlterTableRequest struct {
	Database string `json:"database" binding:"required"`
	Schema   string `json:"schema"`
	Table    string `json:"table" binding:"required"`
	SQL      string `json:"sql" binding:"required"`
}

// ── 用户管理 ──────────────────────────────────────────────────

// PostgresUser 数据库用户（角色）
type PostgresUser struct {
	Name        string `json:"name"`
	SuperUser   bool   `json:"superuser"`
	CreateDB    bool   `json:"create_db"`
	CreateRole  bool   `json:"create_role"`
	CanLogin    bool   `json:"can_login"`
	Replication bool   `json:"replication"`
	ConnLimit   int    `json:"conn_limit"`
	ValidUntil  string `json:"valid_until,omitempty"`
	Members     string `json:"members,omitempty"`
}

// PostgresCreateUserRequest 创建用户请求
type PostgresCreateUserRequest struct {
	Name       string `json:"name" binding:"required"`
	Password   string `json:"password" binding:"required"`
	SuperUser  bool   `json:"superuser"`
	CreateDB   bool   `json:"create_db"`
	CreateRole bool   `json:"create_role"`
	CanLogin   bool   `json:"can_login"`
	ConnLimit  int    `json:"conn_limit"`
	ValidUntil string `json:"valid_until"`
	Grant      string `json:"grant,omitempty"`
}

// PostgresGrantRequest 用户授权请求
type PostgresGrantRequest struct {
	User       string `json:"user" binding:"required"`
	Privileges string `json:"privileges" binding:"required"`
	Object     string `json:"object" binding:"required"`
	ObjectType string `json:"object_type"`
}

// ── 慢查询 ────────────────────────────────────────────────────

// PostgresSlowQuery 慢查询记录（来自 pg_stat_statements）
type PostgresSlowQuery struct {
	Query          string  `json:"query"`
	Calls          int64   `json:"calls"`
	TotalTimeMs    float64 `json:"totalTimeMs"`
	MeanTimeMs     float64 `json:"meanTimeMs"`
	MinTimeMs      float64 `json:"minTimeMs"`
	MaxTimeMs      float64 `json:"maxTimeMs"`
	Rows           int64   `json:"rows"`
	SharedBlksHit  int64   `json:"sharedBlksHit"`
	SharedBlksRead int64   `json:"sharedBlksRead"`
	DBName         string  `json:"dbName,omitempty"`
	UserName       string  `json:"userName,omitempty"`
}

// PostgresSlowQueryList 慢查询列表及 pg_stat_statements 状态
type PostgresSlowQueryList struct {
	PgStatStatements bool                `json:"pgStatStatements"`
	Total            int64               `json:"total"`
	Items            []PostgresSlowQuery `json:"items"`
}
