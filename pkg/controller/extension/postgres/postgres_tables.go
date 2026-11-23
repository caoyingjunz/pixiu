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

package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/caoyingjunz/pixiu/pkg/types"
)

const pgTableSizeSQL = `case when pg_catalog.has_table_privilege(c.oid, 'SELECT')
	then pg_catalog.pg_total_relation_size(c.oid) else 0 end`

const pgTableRowsSQL = `case when c.relkind in ('r', 'm') and c.relispopulated and c.reltuples >= 0
	and pg_catalog.has_table_privilege(c.oid, 'SELECT') and not pg_catalog.row_security_active(c.oid)
	and not exists (select 1 from pg_catalog.pg_inherits i where i.inhparent = c.oid)
	then c.reltuples::bigint end`

func (c *controller) ListTableMetadata(ctx context.Context, id int64, database, schema string, opts types.TableMetadataOptions) ([]string, error) {
	db, _, err := c.connTo(ctx, id, database)
	if err != nil {
		return nil, err
	}
	return listPostgresTableMetadata(ctx, db, schema, opts)
}

func listPostgresTableMetadata(ctx context.Context, db *sql.DB, schema string, opts types.TableMetadataOptions) ([]string, error) {
	ctx, cancel := pgOpContext(ctx)
	defer cancel()

	var query string
	args := []any{schema}
	if opts.Metadata == "columnNames" {
		query = `select column_name from information_schema.columns
where table_schema = $1 and table_name = $2 order by ordinal_position`
		args = append(args, opts.Table)
	} else {
		query = `select c.relname from pg_catalog.pg_class c
join pg_catalog.pg_namespace n on n.oid = c.relnamespace
where n.nspname = $1 and c.relkind in ('r', 'v', 'm', 'S')`
		if opts.Keyword != "" {
			switch opts.Match {
			case "exact":
				query += ` and c.relname::text = $3`
			case "prefix":
				query += ` and strpos(lower(c.relname::text), lower($3)) = 1`
			default:
				query += ` and strpos(lower(c.relname::text), lower($3)) > 0`
			}
		} else {
			query += ` and $3::text = ''`
		}
		limit := opts.Limit
		if limit == 0 {
			limit = 50
		}
		query += ` order by c.relname, c.oid limit $2`
		args = append(args, limit, opts.Keyword)
	}
	names, err := pgQuerySlice(ctx, db, query, func(row *sql.Rows) (string, error) {
		var name string
		err := row.Scan(&name)
		return name, err
	}, args...)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	return names, nil
}

func (c *controller) ListTablePage(ctx context.Context, id int64, database, schema string, opts types.PostgresTablePageOptions) (*types.PostgresTablePage, error) {
	db, _, err := c.connTo(ctx, id, database)
	if err != nil {
		return nil, err
	}
	return listPostgresTablePage(ctx, db, schema, opts)
}

func listPostgresTablePage(ctx context.Context, db *sql.DB, schema string, opts types.PostgresTablePageOptions) (*types.PostgresTablePage, error) {
	ctx, cancel := pgQueryContext(ctx)
	defer cancel()
	if opts.PageSize == 0 {
		opts.PageSize = 10
	}
	result := &types.PostgresTablePage{Items: []types.PostgresTable{}, CountsOmitted: opts.OmitCounts}
	rows, err := db.QueryContext(ctx, pgTablePageSQL(opts), schema, strings.TrimSpace(opts.Keyword),
		opts.FilterField, opts.PageSize, (opts.Page-1)*opts.PageSize)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		table := types.PostgresTable{Type: "table"}
		if err := rows.Scan(&result.Total, &result.TableCount, &table.Schema, &table.Name,
			&table.SizeBytes, &table.Owner, &table.Comment, &table.Rows); err != nil {
			return nil, wrapPgErr(err)
		}
		if table.Name != "" {
			result.Items = append(result.Items, table)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, wrapPgErr(err)
	}
	return result, nil
}

func pgTablePageSQL(opts types.PostgresTablePageOptions) string {
	// 默认排序仅查目录；按体积排序需在 LIMIT 前测量全部匹配表
	sizeExpr, order := "null::bigint", "schema_name, table_name, c.oid"
	rowsSortExpr, rowsExpr := "null::bigint", pgTableRowsSQL
	switch opts.SizeOrder {
	case "asc":
		sizeExpr, order = pgTableSizeSQL, "sort_size asc, "+order
	case "desc":
		sizeExpr, order = pgTableSizeSQL, "sort_size desc, "+order
	default:
		switch opts.RowsOrder {
		case "asc":
			rowsSortExpr, rowsExpr, order = pgTableRowsSQL, "c.sort_rows", "sort_rows asc nulls last, "+order
		case "desc":
			rowsSortExpr, rowsExpr, order = pgTableRowsSQL, "c.sort_rows", "sort_rows desc nulls last, "+order
		}
	}
	totals, source := `counts.total, counts.table_count`, `
		(select (select count(*) from filtered) as total,
			(select count(*) from relations) as table_count) counts
		left join page c on true`
	if opts.OmitCounts {
		totals, source = `0, 0`, `page c`
	}
	return `
		with relations as (
			select c.oid, n.nspname::text as schema_name, c.relname::text as table_name,
				c.relowner, c.relkind, c.relispopulated, c.reltuples
			from information_schema.schemata s
			join pg_catalog.pg_namespace n on n.nspname = s.schema_name::name
			join pg_catalog.pg_class c on c.relnamespace = n.oid
			where s.schema_name not like 'pg_%' and s.schema_name <> 'information_schema'
				and ($1 = '' or s.schema_name::text = $1) and c.relkind = 'r'
		), filtered as (
			select * from relations c
			where $2 = '' or case when $3 = 'all' then (
				strpos(lower(c.table_name), lower($2)) > 0
				or strpos(lower(c.schema_name), lower($2)) > 0
				or strpos(lower(pg_catalog.pg_get_userbyid(c.relowner)::text), lower($2)) > 0
				or strpos(lower(pg_catalog.obj_description(c.oid, 'pg_class')), lower($2)) > 0
			) else strpos(lower(case $3
				when 'schema' then c.schema_name
				when 'owner' then pg_catalog.pg_get_userbyid(c.relowner)::text
				when 'comment' then pg_catalog.obj_description(c.oid, 'pg_class')
				else c.table_name end), lower($2)) > 0 end
		), page as (
			select c.*, ` + sizeExpr + ` as sort_size, ` + rowsSortExpr + ` as sort_rows from filtered c
			order by ` + order + ` limit $4 offset $5
		)
		select ` + totals + `, coalesce(c.schema_name, ''), coalesce(c.table_name, ''),
			case when c.oid is null then 0 else coalesce(c.sort_size, ` + pgTableSizeSQL + `) end,
			coalesce(pg_catalog.pg_get_userbyid(c.relowner)::text, ''),
			coalesce(pg_catalog.obj_description(c.oid, 'pg_class'), ''), ` + rowsExpr + `
		from ` + source + `
		order by ` + order
}
