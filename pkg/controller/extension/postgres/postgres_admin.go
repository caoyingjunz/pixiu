package postgres

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
	maxSQLBytes         = 65536 // 控制台单条 SQL 长度上限
	defaultQueryLimit   = 500   // SELECT 结果缺省行数上限
	maxQueryLimit       = 10000 // SELECT 结果最大行数上限
	maxCellValueRunes   = 4096  // 结果单元格最大显示长度（rune）
	maxBatchStatements  = 50    // 批量执行单次最大语句条数
	maxSlowPageSize     = 100   // 慢查询分页单页上限
	defaultSlowPageSize = 20    // 慢查询缺省页大小
	pgOpTimeout         = 10 * time.Second
	pgQueryTimeout      = 30 * time.Second
)

// readOnlyKeywords 只读语句关键字
var readOnlyKeywords = map[string]struct{}{
	"SELECT": {}, "WITH": {}, "EXPLAIN": {}, "ANALYZE": {}, "SHOW": {},
}

// allowedPrivileges PG GRANT 权限白名单
var allowedPrivileges = map[string]struct{}{
	"SELECT": {}, "INSERT": {}, "UPDATE": {}, "DELETE": {}, "TRUNCATE": {},
	"REFERENCES": {}, "TRIGGER": {}, "ALL": {}, "ALL PRIVILEGES": {},
	"CREATE": {}, "CONNECT": {}, "USAGE": {}, "EXECUTE": {},
}

var (
	pgIdentPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_$]{0,62}$`)
)

// ── SQL 语句解析 ─────────────────────────────────────────────

func pgStripComments(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		switch {
		case strings.HasPrefix(s, "--"):
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

func pgFirstKeyword(sqlText string) string {
	s := pgStripComments(sqlText)
	if strings.HasPrefix(s, "(") {
		return "SELECT"
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func pgIsReadOnly(sqlText string) bool {
	_, ok := readOnlyKeywords[pgFirstKeyword(sqlText)]
	return ok
}

func pgHasMultipleStatements(sqlText string) bool {
	count := 0
	var quote byte
	var inLineComment, inBlockComment bool
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(sqlText) && sqlText[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if ch == '\\' && i+1 < len(sqlText) {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'':
			quote = ch
		case '-':
			if i+1 < len(sqlText) && sqlText[i+1] == '-' {
				inLineComment = true
				i++
			}
		case '/':
			if i+1 < len(sqlText) && sqlText[i+1] == '*' {
				inBlockComment = true
				i++
			}
		case ';':
			if strings.TrimSpace(sqlText[:i]) != "" || count > 0 {
				count++
				if count > 1 {
					return true
				}
			}
		}
	}
	return false
}

type pgStatement struct {
	Text      string
	StartLine int
}

func pgSplitStatements(sqlText string) []pgStatement {
	var (
		stmts          []pgStatement
		startIdx       = -1
		startLine      = 1
		line           = 1
		quote          byte
		inLineComment  bool
		inBlockComment bool
	)
	markStart := func(i int) {
		if startIdx < 0 {
			startIdx = i
			startLine = line
		}
	}
	closeStmt := func(endIdx int) {
		if startIdx < 0 {
			return
		}
		if text := strings.TrimSpace(sqlText[startIdx:endIdx]); text != "" {
			stmts = append(stmts, pgStatement{Text: text, StartLine: startLine})
		}
		startIdx = -1
	}
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if inLineComment {
			if ch == '\n' {
				line++
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '\n' {
				line++
			} else if ch == '*' && i+1 < len(sqlText) && sqlText[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if ch == '\n' {
				line++
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\n':
			line++
		case '\'':
			markStart(i)
			quote = ch
		case '-':
			markStart(i)
			if i+1 < len(sqlText) && sqlText[i+1] == '-' {
				inLineComment = true
				i++
			}
		case '/':
			markStart(i)
			if i+1 < len(sqlText) && sqlText[i+1] == '*' {
				inBlockComment = true
				i++
			}
		case ';':
			closeStmt(i)
		case ' ', '\t', '\r':
		default:
			markStart(i)
		}
	}
	closeStmt(len(sqlText))
	return stmts
}

// ── 标识符工具 ───────────────────────────────────────────────

func pgQuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func pgCheckIdent(kind, s string) error {
	if s == "" || len(s) > 63 {
		return apierrors.NewError(fmt.Errorf("invalid %s name: %q", kind, s), http.StatusBadRequest)
	}
	if !pgIdentPattern.MatchString(s) {
		return apierrors.NewError(fmt.Errorf("invalid %s name: %q", kind, s), http.StatusBadRequest)
	}
	return nil
}

// ── 结果集工具 ───────────────────────────────────────────────

func pgConvertCellValue(v interface{}) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(t) {
			return pgTruncateRunes(string(t))
		}
		return "0x" + hex.EncodeToString(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return t
	}
}

func pgTruncateRunes(s string) string {
	if runes := []rune(s); len(runes) > maxCellValueRunes {
		return string(runes[:maxCellValueRunes]) + "...[truncated]"
	}
	return s
}

func pgFillQueryResult(rows *sql.Rows, limit int64, result *types.PostgresQueryResult) error {
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
		if fetched > limit {
			break
		}
		row := make([]interface{}, len(columns))
		for i, v := range values {
			row[i] = pgConvertCellValue(v)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	result.Truncated = fetched > limit
	return nil
}

func pgRunOnConn(ctx context.Context, conn *sql.Conn, sqlText string, limit int64, readOnly bool) (*types.PostgresQueryResult, error) {
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	result := &types.PostgresQueryResult{Statement: strings.ToLower(pgFirstKeyword(sqlText))}
	start := time.Now()

	if readOnly {
		rows, err := conn.QueryContext(ctx, sqlText)
		if err != nil {
			return nil, wrapPgErr(err)
		}
		defer rows.Close()
		if err := pgFillQueryResult(rows, limit, result); err != nil {
			return nil, wrapPgErr(err)
		}
	} else {
		res, err := conn.ExecContext(ctx, sqlText)
		if err != nil {
			return nil, wrapPgErr(err)
		}
		if affected, err := res.RowsAffected(); err == nil {
			result.Affected = affected
		}
	}
	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

func wrapPgErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "password") || strings.Contains(msg, "authentication"):
		return apierrors.NewError(fmt.Errorf("postgres authentication failed: %v", err), http.StatusUnauthorized)
	case strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied"):
		return apierrors.NewError(fmt.Errorf("postgres permission denied: %v", err), http.StatusForbidden)
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "network is unreachable"):
		return apierrors.NewError(fmt.Errorf("postgres connection failed: %v", err), http.StatusBadGateway)
	default:
		return apierrors.NewError(fmt.Errorf("postgres error: %v", err), http.StatusBadGateway)
	}
}

func pgQueryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, pgQueryTimeout)
}

func pgOpContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, pgOpTimeout)
}

// parsePgVersionNum 解析 server_version_num（如 "101900" -> 101900）
// PG13 = 130000, PG12 = 120000, PG10 = 100000
func parsePgVersionNum(s string) int {
	var v int
	fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v
}

// ── SQL 控制台 ───────────────────────────────────────────────

func (c *controller) ExecuteSQL(ctx context.Context, id int64, q *types.PostgresQueryRequest) (*types.PostgresQueryResult, error) {
	if q == nil || strings.TrimSpace(q.SQL) == "" {
		return nil, apierrors.NewError(fmt.Errorf("sql is required"), http.StatusBadRequest)
	}
	if len(q.SQL) > maxSQLBytes {
		return nil, apierrors.NewError(fmt.Errorf("sql length exceeds limit %d bytes", maxSQLBytes), http.StatusBadRequest)
	}
	if pgHasMultipleStatements(q.SQL) {
		return nil, apierrors.NewError(fmt.Errorf("only a single statement is allowed, use batch execution for multiple statements"), http.StatusBadRequest)
	}

	readOnly := pgIsReadOnly(q.SQL)
	if !readOnly {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
	}

	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := pgQueryContext(ctx)
	defer cancel()

	conn, err := dbConn.Conn(ctx)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer conn.Close()

	// Schema 切换
	if q.Schema != "" {
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+pgQuoteIdent(q.Schema)); err != nil {
			return nil, wrapPgErr(err)
		}
	}

	return pgRunOnConn(ctx, conn, q.SQL, q.Limit, readOnly)
}

func (c *controller) ExecuteBatchSQL(ctx context.Context, id int64, req *types.PostgresBatchRequest) (*types.PostgresBatchResult, error) {
	if req == nil || strings.TrimSpace(req.SQL) == "" {
		return nil, apierrors.NewError(fmt.Errorf("sql is required"), http.StatusBadRequest)
	}
	if len(req.SQL) > maxSQLBytes {
		return nil, apierrors.NewError(fmt.Errorf("sql length exceeds limit %d bytes", maxSQLBytes), http.StatusBadRequest)
	}
	statements := pgSplitStatements(req.SQL)
	if len(statements) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no executable statement found"), http.StatusBadRequest)
	}
	if len(statements) > maxBatchStatements {
		return nil, apierrors.NewError(fmt.Errorf("too many statements: %d, limit %d", len(statements), maxBatchStatements), http.StatusBadRequest)
	}

	// 写语句门禁
	for _, st := range statements {
		if !pgIsReadOnly(st.Text) {
			if err := requireAdmin(ctx); err != nil {
				return nil, err
			}
			break
		}
	}

	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := pgQueryContext(ctx)
	defer cancel()

	conn, err := dbConn.Conn(ctx)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer conn.Close()

	if req.Schema != "" {
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+pgQuoteIdent(req.Schema)); err != nil {
			return nil, wrapPgErr(err)
		}
	}

	batch := &types.PostgresBatchResult{
		Items: make([]types.PostgresBatchItem, 0, len(statements)),
		Total: len(statements),
	}
	for i, st := range statements {
		item := types.PostgresBatchItem{Index: i + 1, StartLine: st.StartLine}
		result, err := pgRunOnConn(ctx, conn, st.Text, req.Limit, pgIsReadOnly(st.Text))
		if err != nil {
			item.Error = err.Error()
			batch.Items = append(batch.Items, item)
			batch.StoppedAt = item.Index
			break
		}
		item.Ok = true
		item.Result = result
		batch.Items = append(batch.Items, item)
	}
	return batch, nil
}

// ── 表详情 ───────────────────────────────────────────────────

func (c *controller) GetTableDetail(ctx context.Context, id int64, database, schema, table string) (*types.PostgresTableDetail, error) {
	if err := pgCheckIdent("schema", schema); err != nil {
		return nil, err
	}
	if err := pgCheckIdent("table", table); err != nil {
		return nil, err
	}

	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := pgOpContext(ctx)
	defer cancel()

	detail := &types.PostgresTableDetail{Name: table, Schema: schema}

	// 表体积与行数估算（顺带取 relkind，供前端区分基表/视图/序列）
	_ = dbConn.QueryRowContext(ctx,
		"select coalesce(c.reltuples,0)::bigint, pg_total_relation_size(c.oid), c.relkind from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname=$1 and c.relname=$2",
		schema, table).Scan(&detail.Rows, &detail.SizeBytes, &detail.RelKind)

	// 列信息（使用 pg_catalog 避免 information_schema 域类型问题）
	colRows, err := dbConn.QueryContext(ctx, `
SELECT a.attname,
       format_type(a.atttypid, a.atttypmod),
       coalesce(t.typname, ''),
       a.attnotnull,
       coalesce(pg_get_expr(d.adbin, d.adrelid), ''),
       a.attnum
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_type t ON t.oid = a.atttypid
LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer colRows.Close()

	// 先查主键列（使用 pg_catalog 避免 information_schema 域类型问题）
	pkCols := map[string]bool{}
	pkRows, err := dbConn.QueryContext(ctx, `
SELECT a.attname
FROM pg_index ix
JOIN pg_attribute a ON a.attrelid = ix.indrelid AND a.attnum = ANY(ix.indkey)
JOIN pg_class c ON c.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relname = $2 AND ix.indisprimary`, schema, table)
	if err == nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var col string
			_ = pkRows.Scan(&col)
			pkCols[col] = true
		}
	}

	// 查列注释
	colComments := map[string]string{}
	commentRows, err := dbConn.QueryContext(ctx, `
SELECT a.attname, coalesce(pg_catalog.col_description(c.oid, a.attnum), '')
FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped`, schema, table)
	if err == nil {
		defer commentRows.Close()
		for commentRows.Next() {
			var name, comment string
			_ = commentRows.Scan(&name, &comment)
			colComments[name] = comment
		}
	}

	for colRows.Next() {
		var col types.PostgresColumn
		var notNull bool
		if err := colRows.Scan(&col.Name, &col.FullType, &col.DataType, &notNull, &col.Default, &col.OrdinalPos); err != nil {
			return nil, wrapPgErr(err)
		}
		col.Nullable = !notNull
		col.IsPrimaryKey = pkCols[col.Name]
		col.Comment = colComments[col.Name]
		detail.Columns = append(detail.Columns, col)
	}

	// 基于已获取的列信息重建 DDL，避免重复查询 pg_attribute/pg_index
	detail.DDL = buildPgTableDDL(schema, table, detail.Columns)

	// 索引信息
	idxRows, err := dbConn.QueryContext(ctx, `
SELECT i.relname, ix.indisunique, ix.indisprimary,
       array_to_string(ARRAY(SELECT pg_get_indexdef(ix.indexrelid, k+1, true) FROM generate_subscripts(ix.indkey,1) AS k ORDER BY k), ', '),
       am.amname, pg_get_indexdef(ix.indexrelid)
FROM pg_index ix
JOIN pg_class c ON c.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_am am ON am.oid = i.relam
WHERE n.nspname = $1 AND c.relname = $2
ORDER BY i.relname`, schema, table)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var idx types.PostgresIndex
		if err := idxRows.Scan(&idx.Name, &idx.IsUnique, &idx.IsPrimary, &idx.Columns, &idx.Type, &idx.Def); err != nil {
			return nil, wrapPgErr(err)
		}
		detail.Indexes = append(detail.Indexes, idx)
	}

	// 外键信息（使用 pg_catalog 避免 information_schema 域类型问题）
	fkRows, err := dbConn.QueryContext(ctx, `
SELECT con.conname,
       pg_get_constraintdef(con.oid)
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relname = $2 AND con.contype = 'f'`, schema, table)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer fkRows.Close()

	for fkRows.Next() {
		var fk types.PostgresForeignKey
		var conDef string
		if err := fkRows.Scan(&fk.Name, &conDef); err != nil {
			return nil, wrapPgErr(err)
		}
		// 解析 pg_get_constraintdef 结果，例如：FOREIGN KEY (col1) REFERENCES schema.table(col2) ON UPDATE CASCADE ON DELETE SET NULL
		fk.Columns = parseFkColumns(conDef)
		fk.RefSchema, fk.RefTable, fk.RefColumns, fk.OnUpdate, fk.OnDelete = parseFkReference(conDef)
		detail.ForeignKeys = append(detail.ForeignKeys, fk)
	}

	return detail, nil
}

// parseFkColumns 从 pg_get_constraintdef 结果中提取本表列名
// 例：FOREIGN KEY (col1, col2) REFERENCES ... => "col1, col2"
func parseFkColumns(conDef string) string {
	// FOREIGN KEY (col1, col2) REFERENCES ...
	start := strings.Index(conDef, "(")
	end := strings.Index(conDef, ")")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return conDef[start+1 : end]
}

// parseFkReference 从 pg_get_constraintdef 结果中解析引用信息
// 例：FOREIGN KEY (col1) REFERENCES mydb.public.users(id) ON UPDATE CASCADE ON DELETE SET NULL
func parseFkReference(conDef string) (refSchema, refTable, refColumns, onUpdate, onDelete string) {
	// 找 REFERENCES 关键字
	refIdx := strings.Index(strings.ToUpper(conDef), "REFERENCES")
	if refIdx < 0 {
		return
	}
	rest := conDef[refIdx+len("REFERENCES"):]
	rest = strings.TrimSpace(rest)

	// 解析 schema.table(columns)
	// 格式可能是: schema.table(col) 或 table(col)
	parenStart := strings.Index(rest, "(")
	if parenStart < 0 {
		return
	}
	parenEnd := strings.Index(rest[parenStart:], ")")
	if parenEnd < 0 {
		return
	}
	parenEnd += parenStart

	refColumns = rest[parenStart+1 : parenEnd]
	refPart := strings.TrimSpace(rest[:parenStart])

	// 解析 schema.table 或 table
	parts := strings.SplitN(refPart, ".", 2)
	if len(parts) == 2 {
		refSchema = strings.Trim(parts[0], "\"")
		refTable = strings.Trim(parts[1], "\"")
	} else {
		refTable = strings.Trim(parts[0], "\"")
	}

	// 解析 ON UPDATE / ON DELETE
	upperRest := strings.ToUpper(rest[parenEnd+1:])
	if idx := strings.Index(upperRest, "ON UPDATE"); idx >= 0 {
		after := rest[parenEnd+1+idx+len("ON UPDATE"):]
		after = strings.TrimSpace(after)
		// 取到下一个 ON 或结尾
		nextOn := strings.Index(strings.ToUpper(after), "ON DELETE")
		if nextOn >= 0 {
			onUpdate = strings.TrimSpace(after[:nextOn])
			onDelete = strings.TrimSpace(after[nextOn+len("ON DELETE"):])
		} else {
			onUpdate = strings.TrimSpace(after)
		}
	}
	if idx := strings.Index(upperRest, "ON DELETE"); idx >= 0 {
		after := rest[parenEnd+1+idx+len("ON DELETE"):]
		onDelete = strings.TrimSpace(after)
	}

	return
}

func buildPgFullType(dataType, udtName string, charLen, numPrec int) string {
	// PG 的 information_schema 对 serial 类型显示为 integer/bigint，通过 udt_name 区分
	switch udtName {
	case "int4":
		return "integer"
	case "int8":
		return "bigint"
	case "int2":
		return "smallint"
	case "float4":
		return "real"
	case "float8":
		return "double precision"
	case "bool":
		return "boolean"
	case "varchar":
		if charLen > 0 {
			return fmt.Sprintf("character varying(%d)", charLen)
		}
		return "character varying"
	case "bpchar":
		if charLen > 0 {
			return fmt.Sprintf("character(%d)", charLen)
		}
		return "character"
	case "numeric":
		if numPrec > 0 {
			return fmt.Sprintf("numeric(%d)", numPrec)
		}
		return "numeric"
	}
	if charLen > 0 && (dataType == "character varying" || dataType == "character") {
		return fmt.Sprintf("%s(%d)", dataType, charLen)
	}
	if numPrec > 0 && dataType == "numeric" {
		return fmt.Sprintf("numeric(%d)", numPrec)
	}
	return dataType
}

// buildPgTableDDL 基于已查询的列信息重建 CREATE TABLE 语句，避免重复查询目录表
func buildPgTableDDL(schema, table string, cols []types.PostgresColumn) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s.%s (\n", pgQuoteIdent(schema), pgQuoteIdent(table)))

	var colDefs, pkCols []string
	for _, col := range cols {
		def := fmt.Sprintf("    %s %s", pgQuoteIdent(col.Name), col.FullType)
		if !col.Nullable {
			def += " NOT NULL"
		}
		if col.Default != "" {
			def += " DEFAULT " + col.Default
		}
		colDefs = append(colDefs, def)
		if col.IsPrimaryKey {
			pkCols = append(pkCols, pgQuoteIdent(col.Name))
		}
	}
	if len(pkCols) > 0 {
		colDefs = append(colDefs, "    PRIMARY KEY ("+strings.Join(pkCols, ", ")+")")
	}

	sb.WriteString(strings.Join(colDefs, ",\n"))
	sb.WriteString("\n)")
	return sb.String()
}

// execDDL 在受控超时内执行写语句并统一包装错误；权限与语句校验由调用方负责
func (c *controller) execDDL(ctx context.Context, id int64, sqlText string) error {
	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return err
	}

	ctx, cancel := pgOpContext(ctx)
	defer cancel()

	if _, err := dbConn.ExecContext(ctx, sqlText); err != nil {
		return wrapPgErr(err)
	}
	return nil
}

func (c *controller) CreateTable(ctx context.Context, id int64, req *types.PostgresCreateTableRequest) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.SQL) == "" {
		return apierrors.NewError(fmt.Errorf("sql is required"), http.StatusBadRequest)
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(req.SQL)), "CREATE") {
		return apierrors.NewError(fmt.Errorf("only CREATE TABLE statements are allowed"), http.StatusBadRequest)
	}
	return c.execDDL(ctx, id, req.SQL)
}

func (c *controller) AlterTable(ctx context.Context, id int64, req *types.PostgresAlterTableRequest) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if req == nil || strings.TrimSpace(req.SQL) == "" {
		return apierrors.NewError(fmt.Errorf("sql is required"), http.StatusBadRequest)
	}
	upper := strings.ToUpper(strings.TrimSpace(req.SQL))
	if !strings.HasPrefix(upper, "ALTER") && !strings.HasPrefix(upper, "CREATE INDEX") && !strings.HasPrefix(upper, "DROP INDEX") {
		return apierrors.NewError(fmt.Errorf("only ALTER TABLE / CREATE INDEX / DROP INDEX statements are allowed"), http.StatusBadRequest)
	}
	return c.execDDL(ctx, id, req.SQL)
}

// ── 用户管理 ─────────────────────────────────────────────────

func (c *controller) ListUsers(ctx context.Context, id int64) ([]types.PostgresUser, error) {
	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := pgOpContext(ctx)
	defer cancel()

	users, err := pgQuerySlice(ctx, dbConn, `
SELECT rolname, rolsuper, rolcreatedb, rolcreaterole, rolcanlogin, rolreplication,
       rolconnlimit, coalesce(rolvaliduntil::text, '')
FROM pg_roles
WHERE rolname NOT LIKE 'pg_%'
ORDER BY rolname`, func(r *sql.Rows) (types.PostgresUser, error) {
		var u types.PostgresUser
		err := r.Scan(&u.Name, &u.SuperUser, &u.CreateDB, &u.CreateRole, &u.CanLogin, &u.Replication, &u.ConnLimit, &u.ValidUntil)
		return u, err
	})
	if err != nil {
		return nil, wrapPgErr(err)
	}
	return users, nil
}

func (c *controller) CreateUser(ctx context.Context, id int64, req *types.PostgresCreateUserRequest) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("request is required"), http.StatusBadRequest)
	}
	if err := pgCheckIdent("role", req.Name); err != nil {
		return err
	}
	if len(req.Password) > 128 {
		return apierrors.NewError(fmt.Errorf("password length exceeds limit 128"), http.StatusBadRequest)
	}

	var opts []string
	if req.SuperUser {
		opts = append(opts, "SUPERUSER")
	} else {
		opts = append(opts, "NOSUPERUSER")
	}
	if req.CreateDB {
		opts = append(opts, "CREATEDB")
	} else {
		opts = append(opts, "NOCREATEDB")
	}
	if req.CreateRole {
		opts = append(opts, "CREATEROLE")
	} else {
		opts = append(opts, "NOCREATEROLE")
	}
	if req.CanLogin {
		opts = append(opts, "LOGIN")
	} else {
		opts = append(opts, "NOLOGIN")
	}
	if req.ConnLimit > 0 {
		opts = append(opts, fmt.Sprintf("CONNECTION LIMIT %d", req.ConnLimit))
	}
	if req.ValidUntil != "" {
		opts = append(opts, fmt.Sprintf("VALID UNTIL '%s'", strings.ReplaceAll(req.ValidUntil, "'", "''")))
	}

	stmt := fmt.Sprintf("CREATE ROLE %s WITH PASSWORD '%s' %s",
		pgQuoteIdent(req.Name),
		strings.ReplaceAll(req.Password, "'", "''"),
		strings.Join(opts, " "))

	return c.execDDL(ctx, id, stmt)
}

func (c *controller) DeleteUser(ctx context.Context, id int64, name string) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if err := pgCheckIdent("role", name); err != nil {
		return err
	}

	stmt := fmt.Sprintf("DROP ROLE %s", pgQuoteIdent(name))
	return c.execDDL(ctx, id, stmt)
}

func (c *controller) GrantRole(ctx context.Context, id int64, req *types.PostgresGrantRequest) error {
	if err := requireAdmin(ctx); err != nil {
		return err
	}
	if req == nil {
		return apierrors.NewError(fmt.Errorf("request is required"), http.StatusBadRequest)
	}
	if err := pgCheckIdent("role", req.User); err != nil {
		return err
	}

	// 校验权限白名单
	privParts := strings.Split(req.Privileges, ",")
	normalized := make([]string, 0, len(privParts))
	for _, p := range privParts {
		key := strings.ToUpper(strings.TrimSpace(p))
		if _, ok := allowedPrivileges[key]; !ok {
			return apierrors.NewError(fmt.Errorf("privilege not allowed: %s", p), http.StatusBadRequest)
		}
		normalized = append(normalized, key)
	}
	privStr := strings.Join(normalized, ", ")

	objType := strings.ToUpper(strings.TrimSpace(req.ObjectType))
	if objType == "" {
		objType = "TABLE"
	}

	var stmt string
	switch objType {
	case "DATABASE":
		stmt = fmt.Sprintf("GRANT %s ON DATABASE %s TO %s",
			privStr, pgQuoteIdent(req.Object), pgQuoteIdent(req.User))
	case "SCHEMA":
		stmt = fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s",
			privStr, pgQuoteIdent(req.Object), pgQuoteIdent(req.User))
	default: // TABLE
		stmt = fmt.Sprintf("GRANT %s ON %s TO %s",
			privStr, req.Object, pgQuoteIdent(req.User))
	}

	return c.execDDL(ctx, id, stmt)
}

// ── 慢查询 ───────────────────────────────────────────────────

func (c *controller) ListSlowQueries(ctx context.Context, id int64, page, pageSize int64, orderBy, orderDir string) (*types.PostgresSlowQueryList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultSlowPageSize
	}
	if pageSize > maxSlowPageSize {
		pageSize = maxSlowPageSize
	}

	dbConn, _, err := c.conn(ctx, id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := pgOpContext(ctx)
	defer cancel()

	result := &types.PostgresSlowQueryList{Items: make([]types.PostgresSlowQuery, 0)}

	// 检查 pg_stat_statements 是否安装
	var hasExt bool
	_ = dbConn.QueryRowContext(ctx, "select exists(select 1 from pg_extension where extname='pg_stat_statements')").Scan(&hasExt)
	result.PgStatStatements = hasExt
	if !hasExt {
		return result, nil
	}

	// 检测 PG 版本，选择正确的列名
	// PG13+ 使用 total_exec_time/mean_exec_time/min_exec_time/max_exec_time
	// PG12 及以下使用 total_time/mean_time/min_time/max_time
	var serverVersion string
	_ = dbConn.QueryRowContext(ctx, "SHOW server_version_num").Scan(&serverVersion)
	usePG13Cols := false
	if v := parsePgVersionNum(serverVersion); v >= 130000 {
		usePG13Cols = true
	}

	// 排序字段白名单
	validOrderBy := map[string]string{
		"total_time": "total",
		"mean_time":  "mean",
		"calls":      "calls",
		"rows":       "rows",
		"max_time":   "max",
		"min_time":   "min",
	}
	colSuffix := "_time"
	if usePG13Cols {
		colSuffix = "_exec_time"
	}
	colName := "total" + colSuffix
	if ob, ok := validOrderBy[strings.ToLower(orderBy)]; ok {
		colName = ob + colSuffix
	}
	dir := "DESC"
	if strings.ToUpper(orderDir) == "ASC" {
		dir = "ASC"
	}

	offset := (page - 1) * pageSize
	minCol := "min" + colSuffix
	maxCol := "max" + colSuffix
	meanCol := "mean" + colSuffix

	query := fmt.Sprintf(`
SELECT s.query, s.calls, s.%s, s.%s, s.%s, s.%s, s.rows,
       s.shared_blks_hit, s.shared_blks_read,
       d.datname as db_name,
       u.usename as user_name
FROM pg_stat_statements s
LEFT JOIN pg_database d ON s.dbid = d.oid
LEFT JOIN pg_user u ON s.userid = u.usesysid
ORDER BY s.%s %s LIMIT $1 OFFSET $2`, colName, meanCol, minCol, maxCol, colName, dir)

	rows, err := dbConn.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer rows.Close()

	for rows.Next() {
		var item types.PostgresSlowQuery
		var dbName, userName sql.NullString
		if err := rows.Scan(&item.Query, &item.Calls, &item.TotalTimeMs, &item.MeanTimeMs,
			&item.MinTimeMs, &item.MaxTimeMs, &item.Rows,
			&item.SharedBlksHit, &item.SharedBlksRead, &dbName, &userName); err != nil {
			return nil, wrapPgErr(err)
		}
		if dbName.Valid {
			item.DBName = dbName.String
		}
		if userName.Valid {
			item.UserName = userName.String
		}
		result.Items = append(result.Items, item)
	}

	// 获取总数
	_ = dbConn.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_statements").Scan(&result.Total)

	return result, rows.Err()
}
