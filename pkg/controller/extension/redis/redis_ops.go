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
	"strconv"
	"strings"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const maxSlowLogCount = 200

func (c *controller) ListSlowLogs(ctx context.Context, datasourceId int64, count int64) ([]types.RedisSlowLogItem, error) {
	if count <= 0 {
		count = 50
	}
	if count > maxSlowLogCount {
		count = maxSlowLogCount
	}
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	logs, err := client.SlowLogGet(ctx, count).Result()
	if err != nil {
		return nil, wrapRedisAdminCmdErr(err, "读取慢日志")
	}
	items := make([]types.RedisSlowLogItem, 0, len(logs))
	for _, item := range logs {
		command := ""
		args := item.Args
		if len(args) > 0 {
			command = args[0]
		}
		items = append(items, types.RedisSlowLogItem{
			ID:         item.ID,
			Timestamp:  item.Time.Unix(),
			DurationUs: item.Duration.Microseconds(),
			Command:    command,
			Args:       args,
			ClientAddr: item.ClientAddr,
			ClientName: item.ClientName,
		})
	}
	return items, nil
}

func (c *controller) ListClients(ctx context.Context, datasourceId int64) ([]types.RedisClientItem, error) {
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()

	raw, err := client.ClientList(ctx).Result()
	if err != nil {
		return nil, wrapRedisAdminCmdErr(err, "读取客户端")
	}
	lines := strings.Split(raw, "\n")
	items := make([]types.RedisClientItem, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := parseClientFields(line)
		items = append(items, types.RedisClientItem{
			ID:     fields["id"],
			Addr:   fields["addr"],
			Name:   fields["name"],
			Age:    parseClientInt(fields["age"]),
			Idle:   parseClientInt(fields["idle"]),
			Flags:  fields["flags"],
			DB:     fields["db"],
			Cmd:    fields["cmd"],
			User:   fields["user"],
			TotMem: parseClientInt(fields["tot-mem"]),
		})
	}
	return items, nil
}

func (c *controller) KillClient(ctx context.Context, datasourceId int64, id string) error {
	if err := requireRedisAdmin(ctx); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, " \t") {
		return apierrors.NewError(fmt.Errorf("client id is required"), http.StatusBadRequest)
	}
	client, _, err := c.clientFor(ctx, datasourceId, nil)
	if err != nil {
		return err
	}
	ctx, cancel := opContext(ctx)
	defer cancel()
	n, err := client.ClientKillByFilter(ctx, "ID", id).Result()
	if err != nil {
		return wrapRedisAdminCmdErr(err, "断开客户端")
	}
	if n == 0 {
		return apierrors.NewError(fmt.Errorf("client(%s) not found", id), http.StatusNotFound)
	}
	return nil
}

func parseClientFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, part := range strings.Fields(line) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	return fields
}

func parseClientInt(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}