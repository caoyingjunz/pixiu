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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/api/server/router/extension/sqlbackup"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type meta struct {
	DatasourceId int64 `uri:"datasourceId"`
}

type databaseOptions struct {
	Database string `form:"database" binding:"required"`
}

type pingMySQLRequest struct {
	DatasourceId int64 `json:"datasource_id"`
	types.MySQLSourceConfig
}

type tableOptions struct {
	Database string `form:"database" binding:"required"`
	Table    string `form:"table" binding:"required"`
}

type userOptions struct {
	User string `form:"user" binding:"required"`
	Host string `form:"host"`
}

type sessionOptions struct {
	Id int64 `form:"id" binding:"required"`
}

// pingMySQLAdhoc 临时探测：请求体直接传连接配置，不依赖已保存的数据源
func (mr *mysqlRouter) pingMySQLAdhoc(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		req pingMySQLRequest
		err error
	)
	if err = c.ShouldBindJSON(&req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().PingAdhoc(c, req.DatasourceId, &req.MySQLSourceConfig); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) pingMySQL(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().Ping(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) getMySQLInfo(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().Info(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) listDatabases(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if c.Query("namesOnly") == "true" {
		r.Result, err = mr.c.Extension().Mysql().ListDatabaseNames(c, m.DatasourceId)
	} else {
		r.Result, err = mr.c.Extension().Mysql().ListDatabases(c, m.DatasourceId)
	}
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) listTables(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts databaseOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if c.Request.URL.Query().Has("metadata") {
		var metadata types.TableMetadataOptions
		if err = c.ShouldBindQuery(&metadata); err != nil {
			httputils.SetFailed(c, r, err)
			return
		}
		r.Result, err = mr.c.Extension().Mysql().ListTableMetadata(c, m.DatasourceId, opts.Database, metadata)
	} else if c.Request.URL.Query().Has("page") {
		var page types.MySQLTablePageOptions
		if err = c.ShouldBindQuery(&page); err != nil {
			httputils.SetFailed(c, r, err)
			return
		}
		r.Result, err = mr.c.Extension().Mysql().ListTablePage(c, m.DatasourceId, opts.Database, page)
	} else {
		r.Result, err = mr.c.Extension().Mysql().ListTables(c, m.DatasourceId, opts.Database)
	}
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) getTableDetail(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts tableOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().GetTableDetail(c, m.DatasourceId, opts.Database, opts.Table); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// executeSQL SQL 控制台：读语句放行，写语句由控制器做管理员门禁
func (mr *mysqlRouter) executeSQL(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLQueryRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().ExecuteSQL(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// executeBatchSQL SQL 控制台批量执行：服务端拆分并逐条执行，遇错即停
func (mr *mysqlRouter) executeBatchSQL(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLExecuteBatchRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().ExecuteBatchSQL(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

const maxLegacyImportBodyBytes = types.MySQLImportMaxBytes + (2 << 20)

// importSQL accepts a raw SQL script and keeps compatibility with multipart and JSON clients.
func (mr *mysqlRouter) importSQL(c *gin.Context) {
	r := httputils.NewResponse()
	contentType := c.ContentType()
	bodyLimit := int64(maxLegacyImportBodyBytes)
	if contentType == "application/octet-stream" {
		bodyLimit = types.MySQLImportMaxBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit)
	defer c.Request.Body.Close()

	var m meta
	if err := c.ShouldBindUri(&m); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}

	var req types.MySQLExecuteBatchRequest
	switch contentType {
	case "application/octet-stream":
		script, readErr := readImportScript(c.Request.Body)
		if readErr != nil {
			httputils.SetFailed(c, r, readErr)
			return
		}
		req = types.MySQLExecuteBatchRequest{Database: c.Query("database"), SQL: script}
	case "multipart/form-data":
		reader, mrErr := c.Request.MultipartReader()
		if mrErr != nil {
			httputils.SetFailed(c, r, apierrors.NewError(fmt.Errorf("invalid multipart request: %v", mrErr), http.StatusBadRequest))
			return
		}
		var hasFile bool
		for {
			part, partErr := reader.NextPart()
			if errors.Is(partErr, io.EOF) {
				break
			}
			if partErr != nil {
				httputils.SetFailed(c, r, importReadError(partErr))
				return
			}
			switch part.FormName() {
			case "database":
				val, readErr := io.ReadAll(io.LimitReader(part, 1024))
				if readErr != nil {
					httputils.SetFailed(c, r, importReadError(readErr))
					return
				}
				_ = part.Close()
				req.Database = string(val)
			case "file":
				hasFile = true
				script, readErr := readImportScript(part)
				if readErr != nil {
					httputils.SetFailed(c, r, readErr)
					return
				}
				req.SQL = script
			default:
				_ = part.Close()
			}
		}
		if !hasFile {
			httputils.SetFailed(c, r, apierrors.NewError(fmt.Errorf("sql file is required"), http.StatusBadRequest))
			return
		}
	default:
		if err := c.ShouldBindJSON(&req); err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				err = importReadError(err)
			}
			httputils.SetFailed(c, r, err)
			return
		}
	}

	result, err := mr.c.Extension().Mysql().ImportSQL(c, m.DatasourceId, &req)
	if err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	r.Result = result
	httputils.SetSuccess(c, r)
}

func readImportScript(body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, types.MySQLImportMaxBytes+1))
	if len(data) > types.MySQLImportMaxBytes {
		err = &http.MaxBytesError{Limit: types.MySQLImportMaxBytes}
	}
	if err != nil {
		return "", importReadError(err)
	}
	return string(data), nil
}

func importReadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apierrors.NewError(fmt.Errorf("sql upload exceeds limit %d bytes", tooLarge.Limit), http.StatusRequestEntityTooLarge)
	}
	return apierrors.NewError(fmt.Errorf("failed to read SQL script: %w", err), http.StatusBadRequest)
}

func (mr *mysqlRouter) listUsers(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().ListUsers(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// createUser 创建实例用户（可附带授权）
func (mr *mysqlRouter) createUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLCreateUserRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = mr.c.Extension().Mysql().CreateUser(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) deleteUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts userOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = mr.c.Extension().Mysql().DeleteUser(c, m.DatasourceId, opts.User, opts.Host); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// grantUser 用户授权（权限与对象均走白名单校验）
func (mr *mysqlRouter) grantUser(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLGrantRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = mr.c.Extension().Mysql().GrantUser(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) listSessions(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		err error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().ListSessions(c, m.DatasourceId); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// killSession 终止指定会话
func (mr *mysqlRouter) killSession(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts sessionOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if err = mr.c.Extension().Mysql().KillSession(c, m.DatasourceId, opts.Id); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) listSlowQueries(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m    meta
		opts types.MySQLSlowQueryOptions
		err  error
	)
	if err = httputils.ShouldBindAny(c, nil, &m, &opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().ListSlowQueries(c, m.DatasourceId, opts); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

// backup 生成库/表的 SQL 文本备份
func (mr *mysqlRouter) backup(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLBackupRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	if r.Result, err = mr.c.Extension().Mysql().Backup(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	httputils.SetSuccess(c, r)
}

func (mr *mysqlRouter) backupFile(c *gin.Context) {
	var m meta
	var req struct {
		types.MySQLBackupRequest
		Filename string `json:"filename"`
	}
	if err := httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, httputils.NewResponse(), err)
		return
	}
	sqlbackup.Serve(c, req.Filename, func(ctx context.Context, dst io.Writer) (*types.SQLBackupSummary, error) {
		return mr.c.Extension().Mysql().WriteBackup(ctx, m.DatasourceId, &req.MySQLBackupRequest, dst)
	})
}

// exportTable 导出表数据为 CSV（流式响应）
func (mr *mysqlRouter) exportTable(c *gin.Context) {
	r := httputils.NewResponse()

	var (
		m   meta
		req types.MySQLTableExportRequest
		err error
	)
	if err = httputils.ShouldBindAny(c, &req, &m, nil); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
	// CSV 响应体由 Controller 流式写出，成功时不再重复设置响应
	if err = mr.c.Extension().Mysql().ExportTableStreaming(c, m.DatasourceId, &req); err != nil {
		httputils.SetFailed(c, r, err)
		return
	}
}
