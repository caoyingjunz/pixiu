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
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	maxSQLBytes        = 65536   // 控制台单条 SQL 长度上限
	defaultQueryLimit  = 500     // SELECT 结果缺省行数上限
	maxQueryLimit      = 10000   // SELECT 结果最大行数上限
	maxCellValueRunes  = 4096    // 结果单元格最大显示长度（rune）
	maxBackupRows      = 50000   // 备份单表最大行数
	defaultBackupRows  = 10000   // 备份单表缺省行数
	maxBackupBytes     = 8 << 20 // 备份 SQL 文本体积上限 8MB
	maxBackupTables    = 200     // 单次备份表数量上限
	maxSessionListSize = 1000    // PROCESSLIST 最大返回条数
)

// readOnlyKeywords 只读语句关键字：控制台普通用户仅可执行这类语句
var readOnlyKeywords = map[string]struct{}{
	"SELECT": {}, "SHOW": {}, "DESC": {}, "DESCRIBE": {}, "EXPLAIN": {}, "WITH": {},
}

// allowedPrivileges GRANT 权限白名单（大写归一后匹配）
var allowedPrivileges = map[string]struct{}{
	"ALL PRIVILEGES": {}, "SELECT": {}, "INSERT": {}, "UPDATE": {}, "DELETE": {},
	"CREATE": {}, "DROP": {}, "ALTER": {}, "INDEX": {}, "REFERENCES": {},
	"CREATE TEMPORARY TABLES": {}, "TRIGGER": {}, "LOCK TABLES": {}, "EXECUTE": {},
	"CREATE VIEW": {}, "SHOW VIEW": {}, "CREATE ROUTINE": {}, "ALTER ROUTINE": {},
	"EVENT": {}, "GRANT OPTION": {}, "USAGE": {},
}

var (
	// 用户名：字母数字与 _.$-，最长 32（MySQL user 列上限）
	mysqlUserPattern = regexp.MustCompile(`^[A-Za-z0-9_.$-]{1,32}$`)
	// host：允许主机名/IP/通配符 %，最长 60
	mysqlHostPattern = regexp.MustCompile(`^[A-Za-z0-9._%:/-]{1,60}$`)
)

// ── SQL 语句解析 ─────────────────────────────────────────────

// stripLeadingComments 去掉语句前导注释（-- / # / /* */），用于关键字识别
func stripLeadingComments(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		switch {
		case strings.HasPrefix(s, "--"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(s, "#"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return ""
		default:
			return s
		}
	}
}

// firstKeyword 提取语句首个关键字（大写）；括号开头按 SELECT 处理
func firstKeyword(sqlText string) string {
	s := stripLeadingComments(sqlText)
	if strings.HasPrefix(s, "(") {
		return "SELECT"
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

// isReadOnlyStatement 判定是否只读语句
func isReadOnlyStatement(sqlText string) bool {
	_, ok := readOnlyKeywords[firstKeyword(sqlText)]
	return ok
}

// hasMultipleStatements 检测是否包含多条语句（忽略引号与注释内的分号），控制台禁止多语句
func hasMultipleStatements(sqlText string) bool {
	count := 0
	var quote byte // 当前所在引号：' " ` ；0 表示不在引号内
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if quote != 0 {
			if ch == '\\' && quote != '`' { // 反斜杠转义（反引号内不生效）
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '-':
			if i+1 < len(sqlText) && sqlText[i+1] == '-' { // 行注释
				if j := strings.IndexByte(sqlText[i:], '\n'); j >= 0 {
					i += j
				} else {
					return count > 1
				}
			}
		case '#':
			if j := strings.IndexByte(sqlText[i:], '\n'); j >= 0 {
				i += j
			} else {
				return count > 1
			}
		case '/':
			if i+1 < len(sqlText) && sqlText[i+1] == '*' { // 块注释
				if j := strings.Index(sqlText[i:], "*/"); j >= 0 {
					i += j + 1
				} else {
					return count > 1
				}
			}
		case ';':
			if strings.TrimSpace(sqlText[:i]) != "" || count > 0 {
				count++
			}
			if count > 1 {
				return true
			}
		}
	}
	return false
}

// ── SQL 控制台 ───────────────────────────────────────────────

// ExecuteSQL 执行单条 SQL：只读语句放行，写/DDL 语句仅管理员可执行。
// 结果按 limit 截断，单元格超长截断保护。
func (c *controller) ExecuteSQL(ctx context.Context, datasourceId int64, req *types.MySQLQueryRequest) (*types.MySQLQueryResult, error) {
	if req == nil || strings.TrimSpace(req.SQL) == "" {
		return nil, apierrors.NewError(fmt.Errorf("sql is required"), http.StatusBadRequest)
	}
	if len(req.SQL) > maxSQLBytes {
		return nil, apierrors.NewError(fmt.Errorf("sql length exceeds limit %d bytes", maxSQLBytes), http.StatusBadRequest)
	}
	if hasMultipleStatements(req.SQL) {
		return nil, apierrors.NewError(fmt.Errorf("only a single statement is allowed per execution"), http.StatusBadRequest)
	}
	if req.Database != "" {
		if err := checkIdentifier("database", req.Database); err != nil {
			return nil, err
		}
	}

	readOnly := isReadOnlyStatement(req.SQL)
	if !readOnly {
		// 写/DDL/管理语句：收敛到管理员
		if err := requireMySQLAdmin(ctx); err != nil {
			return nil, err
		}
	}

	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := queryContext(ctx)
	defer cancel()

	conn, err := dbConn.Conn(ctx)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer conn.Close()

	// 会话级切库：只影响当前连接，不污染连接池其他连接
	if req.Database != "" {
		if _, err := conn.ExecContext(ctx, "USE "+quoteIdent(req.Database)); err != nil {
			return nil, wrapMySQLErr(err)
		}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	result := &types.MySQLQueryResult{Statement: strings.ToLower(firstKeyword(req.SQL))}
	start := time.Now()

	if readOnly {
		rows, err := conn.QueryContext(ctx, req.SQL)
		if err != nil {
			return nil, wrapMySQLErr(err)
		}
		defer rows.Close()
		if err := fillQueryResult(rows, limit, result); err != nil {
			return nil, wrapMySQLErr(err)
		}
	} else {
		res, err := conn.ExecContext(ctx, req.SQL)
		if err != nil {
			return nil, wrapMySQLErr(err)
		}
		if affected, err := res.RowsAffected(); err == nil {
			result.Affected = affected
		}
	}
	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// fillQueryResult 通用结果集扫描：行数上限截断 + 单元格长度截断
func fillQueryResult(rows *sql.Rows, limit int64, result *types.MySQLQueryResult) error {
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	result.Columns = columns

	fetched := int64(0)
	for rows.Next() {
		if fetched >= limit+1 {
			break
		}
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		fetched++
		if fetched > limit { // 多取的一行仅用于判断截断
			break
		}
		row := make([]interface{}, len(columns))
		for i, v := range values {
			row[i] = convertCellValue(v)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	result.Truncated = fetched > limit
	return nil
}

// convertCellValue 驱动返回值转 JSON 友好类型：二进制按 hex，超长文本截断
func convertCellValue(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(t) {
			return truncateRunes(string(t))
		}
		return "0x" + hex.EncodeToString(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case bool:
		if t {
			return "1"
		}
		return "0"
	default:
		return t
	}
}

func truncateRunes(s string) string {
	if runes := []rune(s); len(runes) > maxCellValueRunes {
		return string(runes[:maxCellValueRunes]) + "...[truncated]"
	}
	return s
}

// ── 用户权限管理 ─────────────────────────────────────────────

// checkUserHost 校验用户名与 host 合法性
func checkUserHost(user, host string) error {
	if !mysqlUserPattern.MatchString(user) {
		return apierrors.NewError(fmt.Errorf("invalid mysql user name: %q", user), http.StatusBadRequest)
	}
	if host == "" {
		host = "%"
	}
	if !mysqlHostPattern.MatchString(host) {
		return apierrors.NewError(fmt.Errorf("invalid mysql user host: %q", host), http.StatusBadRequest)
	}
	return nil
}

// quoteUser 用单引号包裹账号片段并转义
func quoteUser(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// ListUsers 实例用户列表（mysql.user），不含密码散列
func (c *controller) ListUsers(ctx context.Context, datasourceId int64) ([]types.MySQLUser, error) {
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	rows, err := dbConn.QueryContext(ctx, "SELECT user, host, account_locked, password_expired FROM mysql.user ORDER BY user, host")
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()

	users := make([]types.MySQLUser, 0)
	for rows.Next() {
		var item types.MySQLUser
		var locked, expired string
		if err := rows.Scan(&item.User, &item.Host, &locked, &expired); err != nil {
			return nil, wrapMySQLErr(err)
		}
		item.AccountLocked = locked == "Y"
		item.PasswordExpired = expired == "Y"
		users = append(users, item)
	}
	return users, wrapRowsErr(rows.Err())
}

// parseGrantSpec 解析授权模板 "SELECT,INSERT ON db.*" 为权限列表与授权对象
func parseGrantSpec(spec string) (privileges, object string, err error) {
	idx := strings.Index(strings.ToUpper(spec), " ON ")
	if idx < 0 {
		return "", "", fmt.Errorf("grant spec must be like \"SELECT,INSERT ON db.*\"")
	}
	return strings.TrimSpace(spec[:idx]), strings.TrimSpace(spec[idx+4:]), nil
}

// validateGrantParts 校验权限列表与授权对象（*.* / db.* / db.table），
// 全部走白名单与标识符校验，拼接时不引入任何用户原文
func validateGrantParts(privileges, object string) (string, string, error) {
	privParts := strings.Split(privileges, ",")
	normalized := make([]string, 0, len(privParts))
	for _, p := range privParts {
		key := strings.ToUpper(strings.TrimSpace(p))
		if _, ok := allowedPrivileges[key]; !ok {
			return "", "", fmt.Errorf("privilege not allowed: %s", p)
		}
		normalized = append(normalized, key)
	}

	objParts := strings.Split(object, ".")
	if len(objParts) != 2 {
		return "", "", fmt.Errorf("grant object must be *.* or db.* or db.table")
	}
	quotedParts := make([]string, 0, 2)
	for _, part := range objParts {
		part = strings.TrimSpace(part)
		if part == "*" {
			quotedParts = append(quotedParts, "*")
			continue
		}
		// 允许已带反引号的写法：剥掉后校验再重新包裹
		part = strings.Trim(part, "`")
		if err := checkIdentifier("grant object", part); err != nil {
			return "", "", fmt.Errorf("invalid grant object part: %s", part)
		}
		quotedParts = append(quotedParts, quoteIdent(part))
	}
	return strings.Join(normalized, ", "), strings.Join(quotedParts, "."), nil
}

// CreateUser 创建用户（可选附带授权）；仅管理员可调用
func (c *controller) CreateUser(ctx context.Context, datasourceId int64, req *types.MySQLCreateUserRequest) error {
	if err := requireMySQLAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("request is required"), http.StatusBadRequest)
	}
	if req.Host == "" {
		req.Host = "%"
	}
	if err := checkUserHost(req.User, req.Host); err != nil {
		return err
	}
	if len(req.Password) > 128 {
		return apierrors.NewError(fmt.Errorf("password length exceeds limit 128"), http.StatusBadRequest)
	}

	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	stmt := fmt.Sprintf("CREATE USER %s@%s IDENTIFIED BY %s",
		quoteUser(req.User), quoteUser(req.Host), quoteUser(req.Password))
	if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
		return wrapMySQLErr(err)
	}

	if strings.TrimSpace(req.Grant) != "" {
		privileges, object, perr := parseGrantSpec(req.Grant)
		if perr != nil {
			return apierrors.NewError(perr, http.StatusBadRequest)
		}
		normPriv, normObj, verr := validateGrantParts(privileges, object)
		if verr != nil {
			return apierrors.NewError(verr, http.StatusBadRequest)
		}
		grant := fmt.Sprintf("GRANT %s ON %s TO %s@%s",
			normPriv, normObj, quoteUser(req.User), quoteUser(req.Host))
		if _, err := dbConn.ExecContext(ctx, grant); err != nil {
			return wrapMySQLErr(err)
		}
	}
	return nil
}

// DeleteUser 删除用户；仅管理员可调用
func (c *controller) DeleteUser(ctx context.Context, datasourceId int64, user, host string) error {
	if err := requireMySQLAdmin(ctx); err != nil {
		return err
	}
	if host == "" {
		host = "%"
	}
	if err := checkUserHost(user, host); err != nil {
		return err
	}
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	stmt := fmt.Sprintf("DROP USER %s@%s", quoteUser(user), quoteUser(host))
	if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
		return wrapMySQLErr(err)
	}
	return nil
}

// GrantUser 用户授权；仅管理员可调用
func (c *controller) GrantUser(ctx context.Context, datasourceId int64, req *types.MySQLGrantRequest) error {
	if err := requireMySQLAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("request is required"), http.StatusBadRequest)
	}
	if req.Host == "" {
		req.Host = "%"
	}
	if err := checkUserHost(req.User, req.Host); err != nil {
		return err
	}
	normPriv, normObj, err := validateGrantParts(req.Privileges, req.Object)
	if err != nil {
		return apierrors.NewError(err, http.StatusBadRequest)
	}
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	stmt := fmt.Sprintf("GRANT %s ON %s TO %s@%s",
		normPriv, normObj, quoteUser(req.User), quoteUser(req.Host))
	if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
		return wrapMySQLErr(err)
	}
	return nil
}

// ── 会话管理 ─────────────────────────────────────────────────

// ListSessions SHOW FULL PROCESSLIST，Info 字段截断保护
func (c *controller) ListSessions(ctx context.Context, datasourceId int64) ([]types.MySQLSession, error) {
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	rows, err := dbConn.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()

	sessions := make([]types.MySQLSession, 0)
	for rows.Next() {
		var item types.MySQLSession
		var db, info sql.NullString
		if err := rows.Scan(&item.Id, &item.User, &item.Host, &db,
			&item.Command, &item.Time, &item.State, &info); err != nil {
			return nil, wrapMySQLErr(err)
		}
		item.DB = db.String
		item.Info = truncateRunes(info.String)
		if int64(len(sessions)) >= maxSessionListSize {
			break
		}
		sessions = append(sessions, item)
	}
	return sessions, wrapRowsErr(rows.Err())
}

// KillSession 终止指定会话；仅管理员可调用
func (c *controller) KillSession(ctx context.Context, datasourceId int64, id int64) error {
	if err := requireMySQLAdmin(ctx); err != nil {
		return err
	}
	if id <= 0 {
		return apierrors.NewError(fmt.Errorf("invalid session id"), http.StatusBadRequest)
	}
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	// KILL 不支持占位符，id 已校验为正整数，拼接安全
	if _, err := dbConn.ExecContext(ctx, fmt.Sprintf("KILL %d", id)); err != nil {
		return wrapMySQLErr(err)
	}
	return nil
}

// ── 慢查询 ───────────────────────────────────────────────────

// ListSlowQueries 分页读取 mysql.slow_log（需 slow_query_log=ON 且 log_output 含 TABLE）；
// 未开启表输出时返回空列表并附带慢日志状态，由前端提示原因与开启方式
func (c *controller) ListSlowQueries(ctx context.Context, datasourceId, page, pageSize int64) (*types.MySQLSlowQueryList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultSlowPageSize
	}
	if pageSize > maxSlowPageSize {
		pageSize = maxSlowPageSize
	}
	offset := (page - 1) * pageSize
	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := opContext(ctx)
	defer cancel()

	vars, err := scanVars(ctx, dbConn, "slow_query_log", "log_output", "slow_query_log_file", "log_queries_not_using_indexes")
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	result := &types.MySQLSlowQueryList{
		SlowQueryLog:              vars["slow_query_log"],
		LogOutput:                 vars["log_output"],
		SlowQueryLogFile:          vars["slow_query_log_file"],
		LogQueriesNotUsingIndexes: vars["log_queries_not_using_indexes"],
		Items:                     make([]types.MySQLSlowQuery, 0),
	}
	if !strings.EqualFold(result.SlowQueryLog, "ON") ||
		!strings.Contains(strings.ToUpper(result.LogOutput), "TABLE") {
		return result, nil
	}

	if err := dbConn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.slow_log").Scan(&result.Total); err != nil {
		return nil, wrapMySQLErr(err)
	}
	if result.Total == 0 {
		return result, nil
	}

	rows, err := dbConn.QueryContext(ctx,
		"SELECT start_time, query_time, lock_time, rows_sent, rows_examined, user_host, db, sql_text "+
			"FROM mysql.slow_log ORDER BY start_time DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var item types.MySQLSlowQuery
		var startTime time.Time
		var queryTime, lockTime, userHost []byte
		var sqlText []byte
		var dbName sql.NullString
		if err := rows.Scan(&startTime, &queryTime, &lockTime,
			&item.RowsSent, &item.RowsExamined, &userHost, &dbName, &sqlText); err != nil {
			return nil, wrapMySQLErr(err)
		}
		item.StartTime = startTime.Format("2006-01-02 15:04:05")
		item.QueryTime = string(queryTime)
		item.LockTime = string(lockTime)
		item.User, item.Host = parseUserHost(string(userHost))
		item.DB = dbName.String
		item.SQLText = truncateRunes(string(sqlText))
		result.Items = append(result.Items, item)
	}
	return result, wrapRowsErr(rows.Err())
}

// parseUserHost 解析 mysql.slow_log.user_host 格式：root[root] @ localhost []
func parseUserHost(raw string) (user, host string) {
	if i := strings.Index(raw, "["); i > 0 {
		user = strings.TrimSpace(raw[:i])
	}
	if i := strings.Index(raw, "] @ "); i >= 0 {
		rest := raw[i+4:]
		if j := strings.Index(rest, " ["); j >= 0 {
			host = strings.TrimSpace(rest[:j])
		} else {
			host = strings.TrimSpace(rest)
		}
	}
	return user, host
}

// ── 备份 ─────────────────────────────────────────────────────

// Backup 生成库/表的 SQL 文本备份（SHOW CREATE TABLE + INSERT），仅管理员可调用。
// 行数与总体积均有上限，超限标记 Truncated。
func (c *controller) Backup(ctx context.Context, datasourceId int64, req *types.MySQLBackupRequest) (*types.MySQLBackupResult, error) {
	if err := requireMySQLAdmin(ctx); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.Database) == "" {
		return nil, apierrors.NewError(fmt.Errorf("database is required"), http.StatusBadRequest)
	}
	if err := checkIdentifier("database", req.Database); err != nil {
		return nil, err
	}
	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = defaultBackupRows
	}
	if maxRows > maxBackupRows {
		maxRows = maxBackupRows
	}

	dbConn, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}

	// 备份可能涉及大量读取，使用独立的长超时上下文
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// 解析目标表：未指定时取整库全部表
	tables := req.Tables
	if len(tables) == 0 {
		rows, err := dbConn.QueryContext(ctx,
			"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME",
			req.Database)
		if err != nil {
			return nil, wrapMySQLErr(err)
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return nil, wrapMySQLErr(err)
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, wrapMySQLErr(err)
		}
		rows.Close()
	}
	if len(tables) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no tables to backup in database %s", req.Database), http.StatusNotFound)
	}
	if len(tables) > maxBackupTables {
		return nil, apierrors.NewError(fmt.Errorf("backup tables exceed limit %d", maxBackupTables), http.StatusBadRequest)
	}
	for _, t := range tables {
		if err := checkIdentifier("table", t); err != nil {
			return nil, err
		}
	}

	result := &types.MySQLBackupResult{
		Database:    req.Database,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		TableNum:    len(tables),
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- Pixiu MySQL Backup\n-- Database: %s\n-- Generated At: %s\n-- Tables: %d\n\n",
		req.Database, result.GeneratedAt, len(tables)))
	sb.WriteString("SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS = 0;\n\n")

	for _, table := range tables {
		fullName := quoteIdent(req.Database) + "." + quoteIdent(table)

		var tableName, ddl string
		if err := dbConn.QueryRowContext(ctx, "SHOW CREATE TABLE "+fullName).Scan(&tableName, &ddl); err != nil {
			return nil, wrapMySQLErr(err)
		}
		sb.WriteString(fmt.Sprintf("-- Table: %s\nDROP TABLE IF EXISTS %s;\n%s;\n\n",
			table, quoteIdent(table), ddl))

		if req.WithData {
			truncated, err := backupTableData(ctx, dbConn, fullName, quoteIdent(table), maxRows, &sb)
			if err != nil {
				return nil, err
			}
			if truncated {
				result.Truncated = true
			}
		}
		if sb.Len() > maxBackupBytes {
			result.Truncated = true
			sb.WriteString("-- [backup truncated: size limit reached]\n")
			break
		}
	}
	sb.WriteString("\nSET FOREIGN_KEY_CHECKS = 1;\n")

	result.Content = sb.String()
	result.SizeBytes = int64(len(result.Content))
	return result, nil
}

// backupTableData 导出单表数据为 INSERT 语句（每 50 行一条），返回是否因行数上限截断
func backupTableData(ctx context.Context, dbConn *sql.DB, fullName, quotedTable string, maxRows int64, sb *strings.Builder) (bool, error) {
	rows, err := dbConn.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT %d", fullName, maxRows+1))
	if err != nil {
		return false, wrapMySQLErr(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return false, wrapMySQLErr(err)
	}

	truncated := false
	var batch []string
	rowCount := int64(0)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("INSERT INTO %s VALUES\n%s;\n", quotedTable, strings.Join(batch, ",\n")))
		batch = batch[:0]
	}
	for rows.Next() {
		if rowCount >= maxRows {
			truncated = true
			break
		}
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false, wrapMySQLErr(err)
		}
		rowCount++
		literals := make([]string, len(cols))
		for i, v := range values {
			literals[i] = sqlLiteral(v)
		}
		batch = append(batch, "("+strings.Join(literals, ", ")+")")
		if len(batch) >= 50 {
			flush()
		}
	}
	if err := rows.Err(); err != nil {
		return false, wrapMySQLErr(err)
	}
	flush()
	if rowCount > 0 {
		sb.WriteString("\n")
	}
	return truncated, nil
}

// sqlLiteral 将驱动返回值转为 SQL 字面量：NULL/二进制 hex/时间格式化/字符串转义
func sqlLiteral(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		if utf8.Valid(t) {
			return quoteSQLString(string(t))
		}
		return "0x" + hex.EncodeToString(t)
	case time.Time:
		return "'" + t.Format("2006-01-02 15:04:05") + "'"
	case bool:
		if t {
			return "1"
		}
		return "0"
	case int64, uint64, float32, float64, int, uint:
		return fmt.Sprintf("%v", t)
	default:
		return quoteSQLString(fmt.Sprintf("%v", t))
	}
}

// quoteSQLString SQL 字符串字面量转义（反斜杠与单引号）
func quoteSQLString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
