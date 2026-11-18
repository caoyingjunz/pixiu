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

package redis

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

var (
	redisConfigNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	redisACLUserRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
	redisACLCommandRe = regexp.MustCompile(`^[+-]?(@?[A-Za-z0-9._|-]+)$`)
)

// Redis 启动期或不可 CONFIG SET 的常见参数。其余参数若实际不可改，由 CONFIG SET 错误回传。
var redisReadonlyConfigs = map[string]struct{}{
	"bind":                 {},
	"port":                 {},
	"tls-port":             {},
	"unixsocket":           {},
	"unixsocketperm":       {},
	"tcp-backlog":          {},
	"daemonize":            {},
	"supervised":           {},
	"pidfile":              {},
	"logfile":              {},
	"syslog-enabled":       {},
	"syslog-ident":         {},
	"syslog-facility":      {},
	"databases":            {},
	"always-show-logo":     {},
	"set-proc-title":       {},
	"proc-title-template":  {},
	"locale-collate":       {},
	"io-threads":           {},
	"io-threads-do-reads":  {},
	"aclfile":              {},
	"tls-cert-file":        {},
	"tls-key-file":         {},
	"tls-key-file-pass":    {},
	"tls-client-cert-file": {},
	"tls-client-key-file":  {},
	"tls-dh-params-file":   {},
	"tls-ca-cert-file":     {},
	"tls-ca-cert-dir":      {},
}

func (c *controller) ListConfig(ctx context.Context, datasourceId int64) ([]types.RedisConfigItem, error) {
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	raw, err := client.ConfigGet(ctx, "*").Result()
	if err != nil {
		return nil, wrapRedisAdminCmdErr(err, "读取配置")
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]types.RedisConfigItem, 0, len(names))
	for _, name := range names {
		items = append(items, types.RedisConfigItem{Name: name, Value: raw[name]})
	}
	return items, nil
}

func (c *controller) SetConfig(ctx context.Context, datasourceId int64, req *types.RedisSetConfigRequest) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("config is required"), http.StatusBadRequest)
	}
	name := strings.TrimSpace(req.Name)
	if !redisConfigNameRe.MatchString(name) {
		return apierrors.NewError(fmt.Errorf("invalid config name"), http.StatusBadRequest)
	}
	if _, ok := redisReadonlyConfigs[strings.ToLower(name)]; ok {
		return apierrors.NewError(fmt.Errorf("parameter %s is read-only and cannot be changed at runtime", name), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()
	if err = client.ConfigSet(ctx, name, req.Value).Err(); err != nil {
		return wrapRedisAdminCmdErr(err, "修改配置")
	}
	return nil
}

func (c *controller) ListACLUsers(ctx context.Context, datasourceId int64) (*types.RedisACLListResult, error) {
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	raw, err := client.Do(ctx, "ACL", "LIST").StringSlice()
	if err != nil {
		if isRedisACLUnsupported(err) {
			return &types.RedisACLListResult{
				Supported: false,
				Message:   aclUnsupportedMessage(err),
				Users:     []types.RedisACLUser{},
			}, nil
		}
		return nil, wrapRedisAdminCmdErr(err, "读取 ACL 用户")
	}
	users := make([]types.RedisACLUser, 0, len(raw))
	for _, line := range raw {
		user, ok := parseACLUser(line)
		if !ok {
			continue
		}
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	return &types.RedisACLListResult{Supported: true, Users: users}, nil
}

func (c *controller) CreateACLUser(ctx context.Context, datasourceId int64, req *types.RedisCreateACLUserRequest) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("user is required"), http.StatusBadRequest)
	}
	name, err := normalizeACLUsername(req.Name)
	if err != nil {
		return err
	}
	if name == "default" {
		return apierrors.NewError(fmt.Errorf("cannot create or replace the default ACL user"), http.StatusBadRequest)
	}
	if !req.NoPass && strings.TrimSpace(req.Password) == "" {
		return apierrors.NewError(fmt.Errorf("password is required unless nopass is set"), http.StatusBadRequest)
	}

	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	exists, err := aclUserExists(ctx, client, name)
	if err != nil {
		return err
	}
	if exists {
		return apierrors.NewError(fmt.Errorf("acl user(%s) already exists", name), http.StatusConflict)
	}

	args := []interface{}{"ACL", "SETUSER", name, "reset"}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if enabled {
		args = append(args, "on")
	} else {
		args = append(args, "off")
	}
	if req.NoPass {
		args = append(args, "nopass")
	} else {
		args = append(args, ">"+req.Password)
	}
	args = append(args, formatACLKeyPattern(req.Keys), "&*")
	commands, err := normalizeACLCommands(req.Commands)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		commands = []string{"+@all"}
	}
	for _, command := range commands {
		args = append(args, command)
	}
	if err = client.Do(ctx, args...).Err(); err != nil {
		return wrapRedisAdminCmdErr(err, "创建 ACL 用户")
	}
	return nil
}

func (c *controller) DeleteACLUser(ctx context.Context, datasourceId int64, username string) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	name, err := normalizeACLUsername(username)
	if err != nil {
		return err
	}
	if name == "default" {
		return apierrors.NewError(fmt.Errorf("cannot delete the default ACL user"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()
	n, err := client.Do(ctx, "ACL", "DELUSER", name).Int64()
	if err != nil {
		return wrapRedisAdminCmdErr(err, "删除 ACL 用户")
	}
	if n == 0 {
		return apierrors.NewError(fmt.Errorf("acl user(%s) not found", name), http.StatusNotFound)
	}
	return nil
}

func (c *controller) GrantACLUser(ctx context.Context, datasourceId int64, req *types.RedisGrantACLUserRequest) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("grant is required"), http.StatusBadRequest)
	}
	name, err := normalizeACLUsername(req.Name)
	if err != nil {
		return err
	}
	commands, err := normalizeACLCommands(req.Commands)
	if err != nil {
		return err
	}
	keyPattern := strings.TrimSpace(req.Keys)
	if len(commands) == 0 && keyPattern == "" {
		return apierrors.NewError(fmt.Errorf("commands or keys is required"), http.StatusBadRequest)
	}

	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	exists, err := aclUserExists(ctx, client, name)
	if err != nil {
		return err
	}
	if !exists {
		return apierrors.NewError(fmt.Errorf("acl user(%s) not found", name), http.StatusNotFound)
	}

	args := []interface{}{"ACL", "SETUSER", name}
	for _, command := range commands {
		args = append(args, command)
	}
	if keyPattern != "" {
		args = append(args, formatACLKeyPattern(keyPattern))
	}
	if err = client.Do(ctx, args...).Err(); err != nil {
		return wrapRedisAdminCmdErr(err, "授权 ACL 用户")
	}
	return nil
}

func aclUserExists(ctx context.Context, client goredis.UniversalClient, name string) (bool, error) {
	raw, err := client.Do(ctx, "ACL", "LIST").StringSlice()
	if err != nil {
		return false, wrapRedisAdminCmdErr(err, "读取 ACL 用户")
	}
	prefix := "user " + name + " "
	for _, line := range raw {
		if line == "user "+name || strings.HasPrefix(line, prefix) {
			return true, nil
		}
	}
	return false, nil
}

func normalizeACLUsername(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !redisACLUserRe.MatchString(name) {
		return "", apierrors.NewError(fmt.Errorf("invalid acl username"), http.StatusBadRequest)
	}
	return name, nil
}

func normalizeACLCommands(commands []string) ([]string, error) {
	out := make([]string, 0, len(commands))
	seen := make(map[string]struct{}, len(commands))
	for _, raw := range commands {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if !redisACLCommandRe.MatchString(item) {
			return nil, apierrors.NewError(fmt.Errorf("invalid acl command %s", raw), http.StatusBadRequest)
		}
		if item[0] != '+' && item[0] != '-' {
			item = "+" + item
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func formatACLKeyPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "*"
	}
	if strings.HasPrefix(pattern, "~") {
		return pattern
	}
	return "~" + pattern
}

func parseACLUser(line string) (types.RedisACLUser, bool) {
	tokens := splitACLTokens(strings.TrimSpace(line))
	if len(tokens) < 2 || tokens[0] != "user" {
		return types.RedisACLUser{}, false
	}
	user := types.RedisACLUser{
		Name:     tokens[1],
		Enabled:  true,
		Keys:     []string{},
		Channels: []string{},
		Flags:    []string{},
	}
	var commands []string
	for _, token := range tokens[2:] {
		switch {
		case token == "on":
			user.Enabled = true
		case token == "off":
			user.Enabled = false
		case token == "nopass":
			user.NoPass = true
			user.Flags = append(user.Flags, token)
		case strings.HasPrefix(token, "~"):
			user.Keys = append(user.Keys, strings.TrimPrefix(token, "~"))
		case strings.HasPrefix(token, "&"):
			user.Channels = append(user.Channels, strings.TrimPrefix(token, "&"))
		case strings.HasPrefix(token, "+") || strings.HasPrefix(token, "-"):
			commands = append(commands, token)
		case strings.HasPrefix(token, ">") || strings.HasPrefix(token, "#") || strings.HasPrefix(token, "!"):
			// 密码哈希不回传
		default:
			user.Flags = append(user.Flags, token)
		}
	}
	user.Commands = strings.Join(commands, " ")
	return user, true
}

func splitACLTokens(s string) []string {
	tokens := make([]string, 0, 16)
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			b.WriteRune(r)
		case r == ' ' && !inQuote:
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

func isRedisACLUnsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "unknown subcommand") ||
		strings.Contains(msg, "disabled") ||
		strings.Contains(msg, "noperm") ||
		strings.Contains(msg, "without the permission")
}

func aclUnsupportedMessage(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "noperm") || strings.Contains(msg, "without the permission"):
		return "当前 Redis 账号没有 ACL 权限，请使用具备 acl 命令权限的账号连接。"
	case strings.Contains(msg, "disabled"):
		return "当前 Redis 已禁用 ACL 命令，无法查看或管理用户权限。"
	default:
		return "当前 Redis 版本不支持 ACL（需要 Redis 6.0 及以上）。"
	}
}

func wrapRedisAdminCmdErr(err error, action string) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unknown command") || strings.Contains(msg, "unknown subcommand"):
		return apierrors.NewError(fmt.Errorf("当前 Redis 不支持该命令，%s失败", action), http.StatusBadRequest)
	case strings.Contains(msg, "noperm") || strings.Contains(msg, "without the permission"):
		return apierrors.NewError(fmt.Errorf("当前 Redis 账号没有权限执行该操作"), http.StatusForbidden)
	case strings.Contains(msg, "disabled"):
		return apierrors.NewError(fmt.Errorf("Redis 已禁用 CONFIG/ACL 命令，%s失败", action), http.StatusBadRequest)
	default:
		return wrapRedisErr(err)
	}
}
