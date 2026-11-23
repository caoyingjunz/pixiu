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
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

func (c *controller) ListTableMetadata(ctx context.Context, id int64, database string, opts types.TableMetadataOptions) ([]string, error) {
	if err := checkIdentifier("database", database); err != nil {
		return nil, err
	}
	db, _, err := c.connFor(ctx, id)
	if err != nil {
		return nil, err
	}
	return listMySQLTableMetadata(ctx, db, database, opts)
}

func listMySQLTableMetadata(ctx context.Context, db *sql.DB, database string, opts types.TableMetadataOptions) ([]string, error) {
	ctx, cancel := opContext(ctx)
	defer cancel()

	var query string
	args := []any{database}
	if opts.Metadata == "columnNames" {
		query = `SELECT COLUMN_NAME FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CAST(TABLE_NAME AS BINARY) = CAST(? AS BINARY)
ORDER BY ORDINAL_POSITION`
		args = append(args, opts.Table, opts.Table)
	} else {
		query = `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`
		if opts.Keyword != "" {
			switch opts.Match {
			case "exact":
				query += ` AND TABLE_NAME = ? AND CAST(TABLE_NAME AS BINARY) = CAST(? AS BINARY)`
				args = append(args, opts.Keyword, opts.Keyword)
			case "prefix":
				query += ` AND LOCATE(LOWER(?), LOWER(TABLE_NAME)) = 1`
				args = append(args, opts.Keyword)
			default:
				query += ` AND LOCATE(LOWER(?), LOWER(TABLE_NAME)) > 0`
				args = append(args, opts.Keyword)
			}
		}
		limit := opts.Limit
		if limit == 0 {
			limit = 50
		}
		query += ` ORDER BY TABLE_NAME, CAST(TABLE_NAME AS BINARY) LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapMySQLErr(err)
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, wrapMySQLErr(err)
		}
		names = append(names, name)
	}
	return names, wrapRowsErr(rows.Err())
}

func (c *controller) ListTablePage(ctx context.Context, datasourceId int64, database string, opts types.MySQLTablePageOptions) (*types.MySQLTablePage, error) {
	if err := checkIdentifier("database", database); err != nil {
		return nil, err
	}
	db, _, err := c.connFor(ctx, datasourceId)
	if err != nil {
		return nil, err
	}
	return listMySQLTablePage(ctx, db, database, opts)
}

func listMySQLTablePage(ctx context.Context, db *sql.DB, database string, opts types.MySQLTablePageOptions) (*types.MySQLTablePage, error) {
	ctx, cancel := opContext(ctx)
	defer cancel()
	if opts.PageSize == 0 {
		opts.PageSize = 10
	}
	filter, order, keyword := mysqlTablePageCriteria(opts)
	const from = ` FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`
	filterArgs := []any{keyword}
	if opts.FilterField == "all" {
		filterArgs = append(filterArgs, keyword, keyword)
	}
	result := &types.MySQLTablePage{Items: []types.MySQLTable{}, CountsOmitted: opts.OmitCounts}
	if !opts.OmitCounts {
		var err error
		if filter == "" {
			err = db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, database).Scan(&result.TableCount)
			result.Total = result.TableCount
		} else {
			err = db.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(CASE WHEN `+filter+` THEN 1 END)`+from,
				append(filterArgs, database)...).Scan(&result.TableCount, &result.Total)
		}
		if err != nil {
			return nil, wrapMySQLErr(err)
		}
	}

	offset := (opts.Page - 1) * opts.PageSize
	if !opts.OmitCounts && offset >= result.Total {
		return result, nil
	}
	where := ` WHERE TABLE_SCHEMA = ?`
	args := []any{database}
	if filter != "" {
		where += ` AND ` + filter
		args = append(args, filterArgs...)
	}
	items, err := queryMySQLTables(ctx, db, mysqlTableSelect+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, opts.PageSize, offset)...)
	if err != nil {
		return nil, err
	}
	result.Items = items
	return result, nil
}

func mysqlTablePageCriteria(opts types.MySQLTablePageOptions) (filter, order, keyword string) {
	order = `TABLE_NAME, CAST(TABLE_NAME AS BINARY)`
	switch opts.SizeOrder {
	case "asc":
		order = `IFNULL(DATA_LENGTH + INDEX_LENGTH, 0) ASC, ` + order
	case "desc":
		order = `IFNULL(DATA_LENGTH + INDEX_LENGTH, 0) DESC, ` + order
	default:
		switch opts.RowsOrder {
		case "asc":
			order = `(IFNULL(TABLE_ROWS, 0) < 0), IFNULL(TABLE_ROWS, 0) ASC, ` + order
		case "desc":
			order = `(IFNULL(TABLE_ROWS, 0) < 0), IFNULL(TABLE_ROWS, 0) DESC, ` + order
		}
	}
	keyword = strings.TrimSpace(opts.Keyword)
	if keyword != "" {
		contains := func(column string) string {
			return `LOCATE(LOWER(?), LOWER(IFNULL(` + column + `, ''))) > 0`
		}
		column := "TABLE_NAME"
		switch opts.FilterField {
		case "engine":
			column = "ENGINE"
		case "comment":
			column = "TABLE_COMMENT"
		case "all":
			filter = `(` + contains(column) + ` OR ` + contains("ENGINE") + ` OR ` + contains("TABLE_COMMENT") + `)`
			return
		}
		filter = contains(column)
	}
	return
}
