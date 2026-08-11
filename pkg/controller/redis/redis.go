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

// Package redis 提供 Redis 中间件管理能力（外部直连，支持单机/哨兵/集群）
package redis

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	// 连接与操作超时保护
	redisDialTimeout     = 5 * time.Second
	redisOpTimeout       = 10 * time.Second
	redisPoolSize        = 10
	redisIdleTimeout     = 5 * time.Minute
	defaultPageSize      = 10
	maxPageSize          = 100
	scanBatchSize        = 1000  // 会话推进时每次 SCAN 的批量
	maxWalkKeys          = 20000 // 单次跳页最多遍历的 key 数（受操作超时约束）
	maxSessionBoundaries = 500   // 会话保留的页边界数（超出后更早的页需重扫）
	sessionTTL           = 10 * time.Minute
	maxStringValueSize   = 4096                   // 字符串值最大返回长度（rune）
	maxCollectionSize    = 100                    // 集合类 key 最多返回的元素数
	maxWriteKeySize      = 512                    // 写入 key 最大长度（rune）
	maxWriteValueSize    = 65536                  // 写入 string 值最大长度（rune）
	maxTTLSeconds        = int64(365 * 24 * 3600) // TTL 上限 1 年
)

type Getter interface {
	Redis() Interface
}

type Interface interface {
	Ping(ctx context.Context, datasourceId int64) (*types.RedisPing, error)
	// PingAdhoc 临时探测（不落库、不缓存连接），用于创建数据源前的连通性验证
	PingAdhoc(ctx context.Context, cfg *types.RedisSourceConfig) (*types.RedisPing, error)
	// db 为逻辑库编号（0-15）；nil 表示使用数据源配置的默认 DB；cluster 模式强制 0
	Info(ctx context.Context, datasourceId int64, db *int) (*types.RedisInfo, error)
	ScanKeys(ctx context.Context, datasourceId int64, db *int, session string, match string, page int64, pageSize int64) (*types.RedisScanResult, error)
	GetKeyDetail(ctx context.Context, datasourceId int64, db *int, key string) (*types.RedisKeyDetail, error)
	// 写操作：新增 key（仅 string）/删除 key/修改 TTL
	CreateKey(ctx context.Context, datasourceId int64, db *int, req *types.RedisCreateKeyRequest) error
	DeleteKey(ctx context.Context, datasourceId int64, db *int, key string) error
	SetKeyTTL(ctx context.Context, datasourceId int64, db *int, key string, ttl int64) error
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory

	mu      sync.Mutex
	clients map[string]*cachedClient

	sessionMu sync.Mutex
	sessions  map[string]*scanSession
}

// scanBoundary 某一页开始前的 SCAN 状态，用于回跳该页
type scanBoundary struct {
	cursor uint64
	carry  []string
	ended  bool
}

// scanSession 服务端 SCAN 会话：保证页边界精确对齐，支持前进/回跳/跳页
type scanSession struct {
	datasourceId int64
	db           int
	match        string
	pageSize     int64
	pageNo       int64 // 已服务的最后一页（1 起）
	cursor       uint64
	carry        []string
	ended        bool
	boundaries   map[int64]*scanBoundary
	lastAccess   time.Time
}

// cachedClient 按数据源+DB 缓存 redis 连接，配置变化（resourceVersion）时重建
type cachedClient struct {
	resourceVersion int64
	fingerprint     string
	client          goredis.UniversalClient
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:       cfg,
		factory:  f,
		clients:  make(map[string]*cachedClient),
		sessions: make(map[string]*scanSession),
	}
}

// resolveDB 解析生效的逻辑库编号：请求值优先，否则用配置默认；cluster 强制 0
func resolveDB(cfg *types.RedisSourceConfig, db *int) (int, error) {
	requested := cfg.DB
	if db != nil {
		requested = *db
	}
	if requested < 0 || requested > 15 {
		return 0, fmt.Errorf("db must be between 0 and 15")
	}
	if cfg.NormalizeMode() == types.RedisModeCluster {
		return 0, nil
	}
	return requested, nil
}

// validateRedisConfig 按部署模式校验连接配置
func validateRedisConfig(cfg *types.RedisSourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("redis config is required")
	}
	// 统一清理地址空白
	for i := range cfg.Addresses {
		cfg.Addresses[i] = strings.TrimSpace(cfg.Addresses[i])
	}
	switch cfg.NormalizeMode() {
	case types.RedisModeSentinel:
		if strings.TrimSpace(cfg.MasterName) == "" {
			return fmt.Errorf("redis sentinel mode requires master_name")
		}
		if len(cfg.Addresses) == 0 {
			return fmt.Errorf("redis sentinel mode requires at least one sentinel address")
		}
	case types.RedisModeCluster:
		if len(cfg.Addresses) == 0 {
			return fmt.Errorf("redis cluster mode requires at least one node address")
		}
		// Redis Cluster 仅支持 db 0
		cfg.DB = 0
	default:
		if strings.TrimSpace(cfg.Address) == "" {
			return fmt.Errorf("redis standalone mode requires address (host:port)")
		}
	}
	return nil
}

// buildRedisClient 按部署模式构造 redis 客户端（调用方负责 Close）
func buildRedisClient(cfg *types.RedisSourceConfig) (goredis.UniversalClient, error) {
	if err := validateRedisConfig(cfg); err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	switch cfg.NormalizeMode() {
	case types.RedisModeSentinel:
		return goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName:       strings.TrimSpace(cfg.MasterName),
			SentinelAddrs:    cfg.Addresses,
			Password:         cfg.Password,
			SentinelPassword: cfg.SentinelPassword,
			DB:               cfg.DB,
			DialTimeout:      redisDialTimeout,
			ReadTimeout:      redisOpTimeout,
			WriteTimeout:     redisOpTimeout,
			PoolSize:         redisPoolSize,
			ConnMaxIdleTime:  redisIdleTimeout,
		}), nil
	case types.RedisModeCluster:
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:           cfg.Addresses,
			Password:        cfg.Password,
			DialTimeout:     redisDialTimeout,
			ReadTimeout:     redisOpTimeout,
			WriteTimeout:    redisOpTimeout,
			PoolSize:        redisPoolSize,
			ConnMaxIdleTime: redisIdleTimeout,
		}), nil
	default:
		return goredis.NewClient(&goredis.Options{
			Addr:            strings.TrimSpace(cfg.Address),
			Password:        cfg.Password,
			DB:              cfg.DB,
			DialTimeout:     redisDialTimeout,
			ReadTimeout:     redisOpTimeout,
			WriteTimeout:    redisOpTimeout,
			PoolSize:        redisPoolSize,
			ConnMaxIdleTime: redisIdleTimeout,
		}), nil
	}
}

// configFingerprint 配置摘要，用于缓存失效判断
func configFingerprint(cfg *types.RedisSourceConfig) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		cfg.NormalizeMode(), cfg.Address, strings.Join(cfg.Addresses, ","),
		cfg.MasterName, cfg.DB, cfg.Password)
}

// clientFor 获取数据源对应的 redis 客户端（带缓存与配置变更失效）
func (c *controller) clientFor(ctx context.Context, datasourceId int64, db *int) (goredis.UniversalClient, *types.RedisSourceConfig, error) {
	object, err := c.factory.Datasource().Get(ctx, datasourceId)
	if err != nil {
		klog.Errorf("failed to get datasource(%d): %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if object == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource not found"), http.StatusNotFound)
	}
	if object.Type != model.DatasourceTypeRedis || object.SubType != model.DatasourceSubTypeRedis {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) is not a redis datasource", datasourceId), http.StatusBadRequest)
	}
	if !object.External {
		return nil, nil, apierrors.NewError(fmt.Errorf("redis datasource only supports external direct connection"), http.StatusBadRequest)
	}

	var cfg types.DatasourceConfig
	if err = cfg.Unmarshal(object.Config); err != nil {
		klog.Errorf("failed to unmarshal datasource(%d) config: %v", datasourceId, err)
		return nil, nil, apierrors.ErrServerInternal
	}
	if cfg.Redis == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) missing redis config", datasourceId), http.StatusBadRequest)
	}
	effectiveDB, err := resolveDB(cfg.Redis, db)
	if err != nil {
		return nil, nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	cfg.Redis.DB = effectiveDB

	c.mu.Lock()
	defer c.mu.Unlock()

	cacheKey := fmt.Sprintf("%d:%d", datasourceId, effectiveDB)
	fingerprint := configFingerprint(cfg.Redis)
	if cached, ok := c.clients[cacheKey]; ok &&
		cached.resourceVersion == object.ResourceVersion &&
		cached.fingerprint == fingerprint {
		return cached.client, cfg.Redis, nil
	}

	// 配置变更或首次连接：关闭旧连接并重建
	if cached, ok := c.clients[cacheKey]; ok {
		_ = cached.client.Close()
		delete(c.clients, cacheKey)
	}

	client, err := buildRedisClient(cfg.Redis)
	if err != nil {
		return nil, nil, err
	}
	c.clients[cacheKey] = &cachedClient{
		resourceVersion: object.ResourceVersion,
		fingerprint:     fingerprint,
		client:          client,
	}
	return client, cfg.Redis, nil
}

// wrapRedisErr 将 redis 操作错误转换为网关类错误（实例不可达/命令失败）
func wrapRedisErr(err error) error {
	return apierrors.NewError(fmt.Errorf("redis request failed: %v", err), http.StatusBadGateway)
}

func opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, redisOpTimeout)
}

func (c *controller) Ping(ctx context.Context, datasourceId int64) (*types.RedisPing, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RedisPing{Address: cfg.DisplayAddress(), DB: cfg.DB}
	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err = client.Ping(ctx).Err(); err != nil {
		result.Message = err.Error()
		return result, nil
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	// 顺带返回版本信息，失败不影响探测结果
	if info, err := client.Info(ctx, "server").Result(); err == nil {
		result.Version = parseInfoField(info, "redis_version")
	}
	return result, nil
}

// PingAdhoc 使用临时连接探测指定配置，探测完成后立即关闭连接
func (c *controller) PingAdhoc(ctx context.Context, cfg *types.RedisSourceConfig) (*types.RedisPing, error) {
	if cfg == nil {
		return nil, apierrors.NewError(fmt.Errorf("redis config is required"), http.StatusBadRequest)
	}

	result := &types.RedisPing{Address: cfg.DisplayAddress(), DB: cfg.DB}
	client, err := buildRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err := client.Ping(ctx).Err(); err != nil {
		result.Message = err.Error()
		return result, nil
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	if info, err := client.Info(ctx, "server").Result(); err == nil {
		result.Version = parseInfoField(info, "redis_version")
	}
	return result, nil
}

func (c *controller) Info(ctx context.Context, datasourceId int64, db *int) (*types.RedisInfo, error) {
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	// INFO + DBSIZE 共用一次 pipeline，省一次 RTT
	pipe := client.Pipeline()
	infoCmd := pipe.Info(ctx)
	dbSizeCmd := pipe.DBSize(ctx)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, wrapRedisErr(err)
	}
	raw, err := infoCmd.Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	dbSize, err := dbSizeCmd.Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}

	info := &types.RedisInfo{
		RedisVersion:     parseInfoField(raw, "redis_version"),
		RedisMode:        parseInfoField(raw, "redis_mode"),
		OS:               parseInfoField(raw, "os"),
		UsedMemoryHuman:  parseInfoField(raw, "used_memory_human"),
		UptimeInSeconds:  parseInfoInt(raw, "uptime_in_seconds"),
		ConnectedClients: parseInfoInt(raw, "connected_clients"),
		KeyspaceHits:     parseInfoInt(raw, "keyspace_hits"),
		KeyspaceMisses:   parseInfoInt(raw, "keyspace_misses"),
		TotalKeys:        dbSize,
		Raw:              raw,
	}
	return info, nil
}

// scanSessionFor 获取/创建会话；上下文（实例/DB/匹配/页大小）变化或会话过期时重置
// 调用方必须持有 c.sessionMu
func (c *controller) scanSessionFor(session string, datasourceId int64, db int, match string, pageSize int64) *scanSession {
	now := time.Now()
	if len(c.sessions) > 64 {
		for k, s := range c.sessions {
			if now.Sub(s.lastAccess) > sessionTTL {
				delete(c.sessions, k)
			}
		}
	}
	sess, ok := c.sessions[session]
	if !ok {
		sess = &scanSession{boundaries: make(map[int64]*scanBoundary)}
		c.sessions[session] = sess
	}
	if sess.datasourceId != datasourceId || sess.db != db || sess.match != match || sess.pageSize != pageSize || now.Sub(sess.lastAccess) > sessionTTL {
		sess.datasourceId = datasourceId
		sess.db = db
		sess.match = match
		sess.pageSize = pageSize
		sess.pageNo = 0
		sess.cursor = 0
		sess.carry = nil
		sess.ended = false
		sess.boundaries = make(map[int64]*scanBoundary)
	}
	sess.lastAccess = now
	return sess
}

// advanceScanPage 会话推进一页，精确返回 pageSize 个 key（carry 保证页边界对齐）
func (c *controller) advanceScanPage(ctx context.Context, client goredis.UniversalClient, sess *scanSession, walked *int64) ([]string, error) {
	keys := sess.carry
	sess.carry = nil
	// carry 超出页大小时，剩余部分留给下一页
	if int64(len(keys)) > sess.pageSize {
		sess.carry = append(sess.carry, keys[sess.pageSize:]...)
		keys = keys[:sess.pageSize]
	}
	for int64(len(keys)) < sess.pageSize && !sess.ended {
		batch, next, err := client.Scan(ctx, sess.cursor, sess.match, scanBatchSize).Result()
		if err != nil {
			return nil, wrapRedisErr(err)
		}
		*walked += int64(len(batch))
		if *walked > maxWalkKeys {
			return nil, apierrors.NewError(fmt.Errorf("page jump too deep: traversed keys exceed limit %d", maxWalkKeys), http.StatusBadRequest)
		}
		need := sess.pageSize - int64(len(keys))
		if int64(len(batch)) > need {
			sess.carry = append(sess.carry, batch[need:]...)
			batch = batch[:need]
		}
		keys = append(keys, batch...)
		sess.cursor = next
		if next == 0 {
			sess.ended = true
		}
	}
	sess.pageNo++
	return keys, nil
}

func (c *controller) evictOldestBoundary(sess *scanSession) {
	var oldest int64 = -1
	for p := range sess.boundaries {
		if oldest == -1 || p < oldest {
			oldest = p
		}
	}
	if oldest != -1 {
		delete(sess.boundaries, oldest)
	}
}

// ScanKeys 会话式分页浏览：page 支持任意跳转（前进受 maxWalkKeys 保护），回跳复用页边界
func (c *controller) ScanKeys(ctx context.Context, datasourceId int64, db *int, session string, match string, page int64, pageSize int64) (*types.RedisScanResult, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(session) == "" {
		return nil, apierrors.NewError(fmt.Errorf("session is required"), http.StatusBadRequest)
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	match = strings.TrimSpace(match)
	if match == "" {
		match = "*"
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	c.sessionMu.Lock()
	sess := c.scanSessionFor(session, datasourceId, cfg.DB, match, pageSize)

	// 定位会话到目标页：有记录边界则回跳；顺序下一页直接推进；其余情况从头重扫
	if b, ok := sess.boundaries[page]; ok {
		sess.cursor = b.cursor
		sess.carry = append([]string(nil), b.carry...)
		sess.ended = b.ended
		sess.pageNo = page - 1
	} else if page != sess.pageNo+1 {
		sess.cursor = 0
		sess.carry = nil
		sess.ended = false
		sess.pageNo = 0
		sess.boundaries = make(map[int64]*scanBoundary)
	}

	var walked int64
	var keys []string
	// 定位后的状态快照，推进失败时回滚，保证会话一致性
	snapCursor, snapCarry, snapEnded, snapPageNo := sess.cursor, sess.carry, sess.ended, sess.pageNo
	for sess.pageNo < page {
		target := sess.pageNo + 1
		sess.boundaries[target] = &scanBoundary{
			cursor: sess.cursor,
			carry:  append([]string(nil), sess.carry...),
			ended:  sess.ended,
		}
		if len(sess.boundaries) > maxSessionBoundaries {
			c.evictOldestBoundary(sess)
		}
		keys, err = c.advanceScanPage(ctx, client, sess, &walked)
		if err != nil {
			sess.cursor, sess.carry, sess.ended, sess.pageNo = snapCursor, snapCarry, snapEnded, snapPageNo
			c.sessionMu.Unlock()
			return nil, err
		}
	}
	hasMore := !sess.ended || len(sess.carry) > 0
	c.sessionMu.Unlock()

	// pipeline 批量读取 TYPE/TTL，避免逐 key 串行往返
	items := make([]types.RedisKeyItem, 0, len(keys))
	if len(keys) > 0 {
		pipe := client.Pipeline()
		typeCmds := make([]*goredis.StatusCmd, len(keys))
		ttlCmds := make([]*goredis.DurationCmd, len(keys))
		for i, key := range keys {
			typeCmds[i] = pipe.Type(ctx, key)
			ttlCmds[i] = pipe.TTL(ctx, key)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, wrapRedisErr(err)
		}
		for i, key := range keys {
			item := types.RedisKeyItem{Key: key, Type: "unknown", TTL: -1}
			if t, err := typeCmds[i].Result(); err == nil {
				item.Type = t
			}
			if ttl, err := ttlCmds[i].Result(); err == nil {
				item.TTL = int64(ttl.Seconds())
				if ttl < 0 {
					item.TTL = int64(ttl) // -1 永不过期 / -2 不存在
				}
			}
			items = append(items, item)
		}
	}

	return &types.RedisScanResult{Page: page, PageSize: pageSize, HasMore: hasMore, Keys: items}, nil
}

func (c *controller) GetKeyDetail(ctx context.Context, datasourceId int64, db *int, key string) (*types.RedisKeyDetail, error) {
	if strings.TrimSpace(key) == "" {
		return nil, apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return nil, wrapRedisErr(err)
	}
	if keyType == "none" {
		return nil, apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}

	detail := &types.RedisKeyDetail{Key: key, Type: keyType, TTL: -1}
	if ttl, err := client.TTL(ctx, key).Result(); err == nil {
		detail.TTL = int64(ttl.Seconds())
		if ttl < 0 {
			detail.TTL = int64(ttl)
		}
	}
	if encoding, err := client.ObjectEncoding(ctx, key).Result(); err == nil {
		detail.Encoding = encoding
	}
	if size, err := client.MemoryUsage(ctx, key).Result(); err == nil {
		detail.SizeBytes = size
	}

	detail.Value, detail.Truncated = readKeyValue(ctx, client, key, keyType)
	return detail, nil
}

// CreateKey 新增 string 类型 key；key 已存在时拒绝（防误覆盖）
func (c *controller) CreateKey(ctx context.Context, datasourceId int64, db *int, req *types.RedisCreateKeyRequest) error {
	if req == nil || strings.TrimSpace(req.Key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	key := strings.TrimSpace(req.Key)
	if runes := []rune(key); len(runes) > maxWriteKeySize {
		return apierrors.NewError(fmt.Errorf("key length exceeds limit %d", maxWriteKeySize), http.StatusBadRequest)
	}
	if runes := []rune(req.Value); int64(len(runes)) > maxWriteValueSize {
		return apierrors.NewError(fmt.Errorf("value length exceeds limit %d", maxWriteValueSize), http.StatusBadRequest)
	}
	if req.TTL > maxTTLSeconds {
		return apierrors.NewError(fmt.Errorf("ttl exceeds limit %d seconds", maxTTLSeconds), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	var expiration time.Duration
	if req.TTL > 0 {
		expiration = time.Duration(req.TTL) * time.Second
	}
	ok, err := client.SetNX(ctx, key, req.Value, expiration).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if !ok {
		return apierrors.NewError(fmt.Errorf("key(%s) already exists", key), http.StatusConflict)
	}
	return nil
}

// DeleteKey 删除指定 key
func (c *controller) DeleteKey(ctx context.Context, datasourceId int64, db *int, key string) error {
	if strings.TrimSpace(key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	n, err := client.Del(ctx, key).Result()
	if err != nil {
		return wrapRedisErr(err)
	}
	if n == 0 {
		return apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}
	return nil
}

// SetKeyTTL 修改 key 过期时间：ttl>=0 设置过期（EXPIRE），ttl=-1 移除过期（PERSIST）
func (c *controller) SetKeyTTL(ctx context.Context, datasourceId int64, db *int, key string, ttl int64) error {
	if strings.TrimSpace(key) == "" {
		return apierrors.NewError(fmt.Errorf("key is required"), http.StatusBadRequest)
	}
	if ttl < -1 || ttl > maxTTLSeconds {
		return apierrors.NewError(fmt.Errorf("ttl must be between -1 and %d", maxTTLSeconds), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, db)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	var ok bool
	if ttl == -1 {
		ok, err = client.Persist(ctx, key).Result()
	} else {
		ok, err = client.Expire(ctx, key, time.Duration(ttl)*time.Second).Result()
	}
	if err != nil {
		return wrapRedisErr(err)
	}
	if !ok {
		return apierrors.NewError(fmt.Errorf("key not found"), http.StatusNotFound)
	}
	return nil
}

// readKeyValue 按类型读取 key 的值，统一做大小/数量截断保护
func readKeyValue(ctx context.Context, client goredis.UniversalClient, key, keyType string) (interface{}, bool) {
	switch keyType {
	case "string":
		val, err := client.Get(ctx, key).Result()
		if err != nil {
			return nil, false
		}
		truncated := false
		if runes := []rune(val); len(runes) > maxStringValueSize {
			val = string(runes[:maxStringValueSize])
			truncated = true
		}
		return val, truncated

	case "hash":
		fields, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, false
		}
		truncated := len(fields) > maxCollectionSize
		result := make(map[string]string, min(len(fields), maxCollectionSize))
		n := 0
		for k, v := range fields {
			if n >= maxCollectionSize {
				break
			}
			result[k] = truncateString(v)
			n++
		}
		return result, truncated

	case "list":
		total, err := client.LLen(ctx, key).Result()
		if err != nil {
			return nil, false
		}
		values, err := client.LRange(ctx, key, 0, maxCollectionSize-1).Result()
		if err != nil {
			return nil, false
		}
		return truncateSlice(values), total > int64(len(values))

	case "set":
		// 用 SSCAN 只取首批，避免一次性加载超大集合
		members, cursor, err := client.SScan(ctx, key, 0, "", maxCollectionSize).Result()
		if err != nil {
			return nil, false
		}
		return truncateSlice(members), cursor != 0 || int64(len(members)) >= maxCollectionSize

	case "zset":
		total, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return nil, false
		}
		items, err := client.ZRangeWithScores(ctx, key, 0, maxCollectionSize-1).Result()
		if err != nil {
			return nil, false
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, fmt.Sprintf("%v (score: %v)", item.Member, item.Score))
		}
		return values, total > int64(len(values))

	default:
		// stream 等类型第一版不提供值预览
		return fmt.Sprintf("(%s 类型暂不支持预览)", keyType), false
	}
}

func truncateString(s string) string {
	if runes := []rune(s); len(runes) > maxStringValueSize {
		return string(runes[:maxStringValueSize])
	}
	return s
}

func truncateSlice(values []string) []string {
	for i := range values {
		values[i] = truncateString(values[i])
	}
	return values
}

// parseInfoField 从 INFO 原始输出中提取字段值
func parseInfoField(raw, field string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(strings.TrimSpace(line), "\r")
		if strings.HasPrefix(line, field+":") {
			return strings.TrimPrefix(line, field+":")
		}
	}
	return ""
}

func parseInfoInt(raw, field string) int64 {
	n, _ := strconv.ParseInt(parseInfoField(raw, field), 10, 64)
	return n
}
