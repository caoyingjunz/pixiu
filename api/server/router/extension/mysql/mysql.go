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

package mysql

import (
	"github.com/caoyingjunz/pixiu/api/server/router/apiregistry"
	"github.com/caoyingjunz/pixiu/cmd/app/options"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

// mysqlRouter is a router to talk with the mysql controller
type mysqlRouter struct {
	c controller.PixiuInterface
}

// RegisterMysql 将 MySQL 子模块路由注册到 extension 父路由组下，
// 完整路径为 /pixiu/extension/mysql/...
func RegisterMysql(o *options.Options, group *apiregistry.Group) {
	mr := &mysqlRouter{
		c: o.Controller,
	}
	group.Entries = append(group.Entries,
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/ping", Handler: mr.pingMySQLAdhoc, Description: "MySQL 临时连通性探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/ping", Handler: mr.pingMySQL, Description: "MySQL 连接探测"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/info", Handler: mr.getMySQLInfo, Description: "MySQL 实例概览"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/databases", Handler: mr.listDatabases, Description: "MySQL 数据库列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/tables", Handler: mr.listTables, Description: "MySQL 表列表"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/table", Handler: mr.getTableDetail, Description: "MySQL 表详情"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/query", Handler: mr.executeSQL, Description: "MySQL SQL 控制台"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/query/batch", Handler: mr.executeBatchSQL, Description: "MySQL SQL 控制台批量执行"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/users", Handler: mr.listUsers, Description: "MySQL 用户列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/users", Handler: mr.createUser, Description: "MySQL 创建用户"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/mysql/:datasourceId/users", Handler: mr.deleteUser, Description: "MySQL 删除用户"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/grant", Handler: mr.grantUser, Description: "MySQL 用户授权"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/sessions", Handler: mr.listSessions, Description: "MySQL 会话列表"},
		apiregistry.RouteEntry{Method: "DELETE", RelativePath: "/mysql/:datasourceId/sessions", Handler: mr.killSession, Description: "MySQL 终止会话"},
		apiregistry.RouteEntry{Method: "GET", RelativePath: "/mysql/:datasourceId/slowqueries", Handler: mr.listSlowQueries, Description: "MySQL 慢查询列表"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/backup", Handler: mr.backup, Description: "MySQL SQL 文本备份"},
		apiregistry.RouteEntry{Method: "POST", RelativePath: "/mysql/:datasourceId/export/table/direct", Handler: mr.exportTable, Description: "MySQL 表数据 CSV 导出"},
	)
}
