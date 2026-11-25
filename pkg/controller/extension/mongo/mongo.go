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

// Package mongo 提供 MongoDB 中间件管理能力（外部直连）。
package mongo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	datasourcecontroller "github.com/caoyingjunz/pixiu/pkg/controller/datasource"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	mongoConnectTimeout = 5 * time.Second
	mongoOpTimeout      = 10 * time.Second
	maxCachedClients    = 64
	maxSlowLogItems     = 100
)

type Interface interface {
	Ping(ctx context.Context, datasourceId int64) (*types.MongoPing, error)
	PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.MongoSourceConfig) (*types.MongoPing, error)
	Info(ctx context.Context, datasourceId int64) (*types.MongoInfo, error)
	ReplicaSet(ctx context.Context, datasourceId int64) (*types.MongoReplSetResult, error)
	SlowLogs(ctx context.Context, datasourceId int64, count int64) (*types.MongoSlowLogResult, error)
	Params(ctx context.Context, datasourceId int64) (*types.MongoParamResult, error)
}

type controller struct {
	cc      config.Config
	factory db.ShareDaoFactory

	mu      sync.Mutex
	clients map[string]*cachedClient
}

// cachedClient 按数据源缓存 mongo 连接，配置变化（resourceVersion/fingerprint）时重建
type cachedClient struct {
	resourceVersion int64
	fingerprint     string
	client          *mongo.Client
	lastAccess      time.Time
}

func New(cfg config.Config, f db.ShareDaoFactory) Interface {
	return &controller{
		cc:      cfg,
		factory: f,
		clients: make(map[string]*cachedClient),
	}
}

// requireMongoAdmin 限制临时探测仅超管与内置管理员可执行（请求体可指定任意地址，属 SSRF 面）
func requireMongoAdmin(ctx context.Context) error {
	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return err
	}
	if user.Role != model.RoleRoot && user.Role != model.RoleAdmin {
		return apierrors.ErrForbidden
	}
	return nil
}

func validateMongoDatasource(object *model.Datasource) error {
	if object.Type != model.DatasourceTypeMiddleware || object.SubType != model.DatasourceSubTypeMongoDB {
		return apierrors.NewError(fmt.Errorf("datasource(%d) is not a mongodb datasource", object.Id), http.StatusBadRequest)
	}
	if !object.External {
		return apierrors.NewError(fmt.Errorf("mongodb datasource only supports external direct connection"), http.StatusBadRequest)
	}
	return nil
}

// buildMongoURI 由配置构造 MongoDB 连接串（凭据做 URL 编码，避免特殊字符破坏 URI）
func buildMongoURI(cfg *types.MongoSourceConfig) (string, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return "", fmt.Errorf("mongodb host is required")
	}
	hosts := strings.Split(cfg.Host, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
		if hosts[i] == "" {
			return "", fmt.Errorf("mongodb host contains empty entry")
		}
		if !strings.Contains(hosts[i], ":") {
			hosts[i] = fmt.Sprintf("%s:%d", hosts[i], cfg.NormalizePort())
		}
	}

	scheme := "mongodb"
	if cfg.SSL {
		scheme = "mongodb+srv"
	}
	// mongodb+srv 仅支持单一主机名，多节点回退到标准协议
	if cfg.SSL && len(hosts) > 1 {
		scheme = "mongodb"
	}
	uri := scheme + "://" + strings.Join(hosts, ",")

	params := make([]string, 0, 6)
	if cfg.ReplicaSet != "" {
		params = append(params, "replicaSet="+url.QueryEscape(cfg.ReplicaSet))
	}
	authSource := cfg.AuthSource
	if authSource == "" {
		authSource = cfg.NormalizeDatabase()
	}
	params = append(params, "authSource="+url.QueryEscape(authSource))
	if cfg.SSL {
		params = append(params, "tls=true")
	}
	timeoutSec := cfg.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	params = append(params, fmt.Sprintf("connectTimeoutMS=%d", timeoutSec*1000))
	uri += "/?" + strings.Join(params, "&")

	if cfg.UserName != "" {
		// 将凭据注入 host 之前，形如 mongodb://user:pass@host1,host2/?
		cred := url.UserPassword(cfg.UserName, cfg.Password).String()
		uri = scheme + "://" + cred + "@" + strings.Join(hosts, ",") + "/?" + strings.Join(params, "&")
	}
	return uri, nil
}

func buildMongoClient(cfg *types.MongoSourceConfig) (*mongo.Client, error) {
	uri, err := buildMongoURI(cfg)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("mongodb connect failed: %v", err), http.StatusBadGateway)
	}
	return client, nil
}

func mongoFingerprint(cfg *types.MongoSourceConfig) string {
	return fmt.Sprintf("%s|%d|%s|%s|%s|%s|%t|%d",
		cfg.Host, cfg.Port, cfg.UserName, cfg.Database, cfg.ReplicaSet, cfg.AuthSource, cfg.SSL, cfg.Timeout)
}

// clientFor 获取数据源对应的 mongo 客户端（带缓存与配置变更失效）
func (c *controller) clientFor(ctx context.Context, datasourceId int64) (*mongo.Client, *types.MongoSourceConfig, error) {
	object, saved, err := datasourcecontroller.LoadDatasourceConfig(ctx, c.factory, datasourceId, validateMongoDatasource)
	if err != nil {
		return nil, nil, err
	}
	if saved.Mongo == nil {
		return nil, nil, apierrors.NewError(fmt.Errorf("datasource(%d) missing mongo config", datasourceId), http.StatusBadRequest)
	}
	cfg := saved.Mongo
	cacheKey := fmt.Sprintf("%d", datasourceId)
	fingerprint := mongoFingerprint(cfg)

	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.clients[cacheKey]; ok &&
		cached.resourceVersion == object.ResourceVersion && cached.fingerprint == fingerprint {
		cached.lastAccess = time.Now()
		return cached.client, cfg, nil
	}
	if cached, ok := c.clients[cacheKey]; ok {
		_ = cached.client.Disconnect(context.Background())
		delete(c.clients, cacheKey)
	}

	client, err := buildMongoClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	c.clients[cacheKey] = &cachedClient{
		resourceVersion: object.ResourceVersion,
		fingerprint:     fingerprint,
		client:          client,
		lastAccess:      time.Now(),
	}
	if len(c.clients) > maxCachedClients {
		var oldestKey string
		var oldest time.Time
		for k, cc := range c.clients {
			if oldestKey == "" || cc.lastAccess.Before(oldest) {
				oldestKey, oldest = k, cc.lastAccess
			}
		}
		if oldestKey != "" {
			_ = c.clients[oldestKey].client.Disconnect(context.Background())
			delete(c.clients, oldestKey)
		}
	}
	return client, cfg, nil
}

func opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, mongoOpTimeout)
}

// wrapMongoErr 将 mongo 错误映射为带语义的错误码。
// 注意：上游认证失败不能返回 401（前端全局拦截器会将 401 当作登录失效并登出），统一映射为 502。
func wrapMongoErr(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "authentication failed") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "autherror"):
		return apierrors.NewError(fmt.Errorf("mongodb authentication failed: %v", err), http.StatusBadGateway)
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "server selection") ||
		strings.Contains(msg, "no reachable servers") || strings.Contains(msg, "network is unreachable"):
		return apierrors.NewError(fmt.Errorf("mongodb connection failed: %v", err), http.StatusBadGateway)
	default:
		return apierrors.NewError(fmt.Errorf("mongodb request failed: %v", err), http.StatusBadGateway)
	}
}

// pingMongoClient 执行 Ping 并顺带读取版本，Ping 与 PingAdhoc 共用
func pingMongoClient(ctx context.Context, client *mongo.Client, cfg *types.MongoSourceConfig) *types.MongoPing {
	result := &types.MongoPing{Address: cfg.DisplayAddress()}
	ctx, cancel := opContext(ctx)
	defer cancel()

	start := time.Now()
	if err := client.Ping(ctx, nil); err != nil {
		result.Message = err.Error()
		return result
	}
	result.Connected = true
	result.LatencyMs = time.Since(start).Milliseconds()

	var buildInfo struct {
		Version string `bson:"version"`
	}
	if err := client.Database(cfg.NormalizeDatabase()).RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&buildInfo); err == nil {
		result.Version = buildInfo.Version
	}
	return result
}

func (c *controller) Ping(ctx context.Context, datasourceId int64) (*types.MongoPing, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	return pingMongoClient(ctx, client, cfg), nil
}

// PingAdhoc 使用临时配置探测连通性，探测完成后立即断开。仅管理员可调用。
func (c *controller) PingAdhoc(ctx context.Context, datasourceId int64, cfg *types.MongoSourceConfig) (*types.MongoPing, error) {
	if err := requireMongoAdmin(ctx); err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &types.MongoSourceConfig{}
	}
	if datasourceId > 0 {
		// 编辑已保存数据源时前端密码字段为空，回填服务端保存的密码
		_, saved, err := datasourcecontroller.LoadDatasourceConfigForAdhoc(ctx, c.factory, datasourceId, validateMongoDatasource, "MongoDB")
		if err != nil {
			return nil, err
		}
		if saved.Mongo == nil {
			return nil, apierrors.NewError(fmt.Errorf("invalid MongoDB datasource config"), http.StatusBadRequest)
		}
		if cfg.Host == "" {
			cfg.Host = saved.Mongo.Host
		}
		if cfg.Port == 0 {
			cfg.Port = saved.Mongo.Port
		}
		if cfg.UserName == "" {
			cfg.UserName = saved.Mongo.UserName
		}
		if cfg.Password == "" {
			cfg.Password = saved.Mongo.Password
		}
		if cfg.Database == "" {
			cfg.Database = saved.Mongo.Database
		}
		if cfg.ReplicaSet == "" {
			cfg.ReplicaSet = saved.Mongo.ReplicaSet
		}
		if cfg.AuthSource == "" {
			cfg.AuthSource = saved.Mongo.AuthSource
		}
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, apierrors.NewError(fmt.Errorf("mongodb host is required"), http.StatusBadRequest)
	}
	client, err := buildMongoClient(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	return pingMongoClient(ctx, client, cfg), nil
}

// intFromAny 将 bson 解码出的数值（int32/int64/float64/double）统一转为 int64
func intFromAny(v any) int64 {
	switch n := v.(type) {
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}

func strFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (c *controller) Info(ctx context.Context, datasourceId int64) (*types.MongoInfo, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	dbName := cfg.NormalizeDatabase()
	var serverStatus struct {
		Version     string `bson:"version"`
		Uptime      int64  `bson:"uptime"`
		Host        string `bson:"host"`
		ReplSet     string `bson:"replSet"`
		Connections struct {
			Current      int64 `bson:"current"`
			Available    int64 `bson:"available"`
			TotalCreated int64 `bson:"totalCreated"`
		} `bson:"connections"`
		OpCounters struct {
			Insert  int64 `bson:"insert"`
			Query   int64 `bson:"query"`
			Update  int64 `bson:"update"`
			Delete  int64 `bson:"delete"`
			Command int64 `bson:"command"`
			GetMore int64 `bson:"getmore"`
		} `bson:"opcounters"`
		Network struct {
			BytesIn  int64 `bson:"bytesIn"`
			BytesOut int64 `bson:"bytesOut"`
		} `bson:"network"`
	}
	if err := client.Database(dbName).RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&serverStatus); err != nil {
		return nil, wrapMongoErr(err)
	}

	var dbStats struct {
		DataSize    float64 `bson:"dataSize"`
		StorageSize float64 `bson:"storageSize"`
		IndexSize   float64 `bson:"indexSize"`
		Collections int64   `bson:"collections"`
		Objects     int64   `bson:"objects"`
	}
	// dbStats 对 admin 库同样可用；失败不影响概览主体
	_ = client.Database(dbName).RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&dbStats)

	info := &types.MongoInfo{
		Version:       serverStatus.Version,
		UptimeSeconds: serverStatus.Uptime,
		Host:          serverStatus.Host,
		ReplicaSet:    serverStatus.ReplSet,
		CurrentOp: serverStatus.OpCounters.Insert + serverStatus.OpCounters.Query +
			serverStatus.OpCounters.Update + serverStatus.OpCounters.Delete + serverStatus.OpCounters.Command,
		Connections:    serverStatus.Connections.Current,
		AvailableConns: serverStatus.Connections.Available,
		TotalCreated:   serverStatus.Connections.TotalCreated,
		DataSize:       int64(dbStats.DataSize),
		StorageSize:    int64(dbStats.StorageSize),
		IndexSize:      int64(dbStats.IndexSize),
		Collections:    dbStats.Collections,
		Objects:        dbStats.Objects,
		DbName:         dbName,
		Insert:         serverStatus.OpCounters.Insert,
		Query:          serverStatus.OpCounters.Query,
		Update:         serverStatus.OpCounters.Update,
		Delete:         serverStatus.OpCounters.Delete,
		Command:        serverStatus.OpCounters.Command,
		GetMore:        serverStatus.OpCounters.GetMore,
		BytesIn:        serverStatus.Network.BytesIn,
		BytesOut:       serverStatus.Network.BytesOut,
	}
	return info, nil
}

func (c *controller) ReplicaSet(ctx context.Context, datasourceId int64) (*types.MongoReplSetResult, error) {
	client, _, err := c.clientFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	var raw bson.M
	err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&raw)
	if err != nil {
		// 单机实例不支持副本集命令，返回 supported=false 而非错误
		return &types.MongoReplSetResult{
			Supported: false,
			Message:   "当前实例未启用副本集（replSetGetStatus 不可用）",
		}, nil
	}

	result := &types.MongoReplSetResult{Supported: true, SetName: strFromAny(raw["set"])}
	if members, ok := raw["members"].(bson.A); ok {
		for _, m := range members {
			memberMap, ok := m.(bson.M)
			if !ok {
				continue
			}
			optime := ""
			if ot, ok := memberMap["optimeDate"].(time.Time); ok {
				optime = ot.Format("2006-01-02 15:04:05")
			}
			result.Members = append(result.Members, types.MongoReplSetMember{
				Name:       strFromAny(memberMap["name"]),
				StateStr:   strFromAny(memberMap["stateStr"]),
				Health:     int(intFromAny(memberMap["health"])),
				UptimeSec:  intFromAny(memberMap["uptime"]),
				PingMs:     intFromAny(memberMap["pingMs"]),
				OptimeDate: optime,
				Self:       memberMap["self"] == true,
			})
		}
	}
	// 从成员中推断当前节点角色
	for _, m := range result.Members {
		if m.Self {
			result.MyState = m.StateStr
			break
		}
	}
	return result, nil
}

func (c *controller) SlowLogs(ctx context.Context, datasourceId int64, count int64) (*types.MongoSlowLogResult, error) {
	client, cfg, err := c.clientFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()
	if count <= 0 || count > maxSlowLogItems {
		count = maxSlowLogItems
	}

	db := client.Database(cfg.NormalizeDatabase())
	coll := db.Collection("system.profile")
	cur, err := coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(count))
	if err != nil {
		// profiling 未开启时 system.profile 不存在，返回 supported=false
		return &types.MongoSlowLogResult{Supported: false, Message: "目标实例未开启 Profiling（system.profile 不可用）"}, nil
	}
	defer func() { _ = cur.Close(ctx) }()

	items := make([]types.MongoSlowLogItem, 0, count)
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		ts := ""
		if t, ok := doc["ts"].(time.Time); ok {
			ts = t.Format("2006-01-02 15:04:05")
		}
		command := ""
		if cmd, ok := doc["command"].(bson.M); ok {
			parts := make([]string, 0, len(cmd))
			for k := range cmd {
				parts = append(parts, k)
			}
			command = strings.Join(parts, " ")
		} else {
			command = strFromAny(doc["command"])
		}
		items = append(items, types.MongoSlowLogItem{
			Timestamp:    ts,
			DurationMs:   intFromAny(doc["millis"]),
			Op:           strFromAny(doc["op"]),
			Namespace:    strFromAny(doc["ns"]),
			Command:      command,
			PlanSummary:  strFromAny(doc["planSummary"]),
			DocsExamined: intFromAny(doc["docsExamined"]),
			NReturned:    intFromAny(doc["nreturned"]),
			Client:       strFromAny(doc["client"]),
		})
	}
	return &types.MongoSlowLogResult{Supported: true, Items: items}, nil
}

func (c *controller) Params(ctx context.Context, datasourceId int64) (*types.MongoParamResult, error) {
	client, _, err := c.clientFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	var raw bson.M
	err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "getParameter", Value: 1}}).Decode(&raw)
	if err != nil {
		// 部分版本需要 getParameter:"*"；再尝试一次
		err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "getParameter", Value: "*"}}).Decode(&raw)
		if err != nil {
			return &types.MongoParamResult{Supported: false, Message: "当前实例不支持 getParameter 命令"}, nil
		}
	}

	items := make([]types.MongoParamItem, 0, len(raw))
	for k, v := range raw {
		if k == "ok" {
			continue
		}
		items = append(items, types.MongoParamItem{Name: k, Value: fmt.Sprintf("%v", v)})
	}
	return &types.MongoParamResult{Supported: true, Items: items}, nil
}
