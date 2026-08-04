/*
Copyright 2024 The Pixiu Authors.

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

package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormmigrator "gorm.io/gorm/migrator"
)

// OpenPostgres 打开 PostgreSQL / 人大金仓（PG 协议）连接。
//
//  1. PreferSimpleProtocol：金仓时间 OID（如 7954）与标准 PG 不同，扩展协议编码 time.Time 会失败。
//  2. AfterConnect 注册金仓日期时间 OID：否则回读时驱动把时间当成 string，
//     Scan 到 *time.Time 会报 unsupported Scan ... type string。
//  3. 兼容 Migrator：ColumnTypes 遇 udt_name 缺失时回退 data_type。
func OpenPostgres(dsn string) (gorm.Dialector, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	sqlDB := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(registerKingbaseDateTimeTypes))
	return &compatiblePostgresDialector{
		Dialector: postgres.New(postgres.Config{
			Conn:                 sqlDB,
			PreferSimpleProtocol: true,
		}).(*postgres.Dialector),
	}, nil
}

// registerKingbaseDateTimeTypes 将金仓/非标准日期时间 OID 注册到 pgx，保证编解码走 timestamp 而非裸 string。
func registerKingbaseDateTimeTypes(ctx context.Context, conn *pgx.Conn) error {
	tm := conn.TypeMap()
	// 启动创建用户时曾出现的金仓 timestamp OID
	registerDateTimeOID(tm, 7954, "timestamp")

	rows, err := conn.Query(ctx, `
		SELECT oid::int, typname
		FROM pg_catalog.pg_type
		WHERE typcategory = 'D'
		   OR typname IN ('timestamp', 'timestamptz', 'date', 'time', 'timetz', 'datetime', 'smalldatetime')
	`)
	if err != nil {
		// 目录查询失败不阻断连接（例如权限/兼容模式差异），保留上面的硬编码 OID
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var oid uint32
		var name string
		if scanErr := rows.Scan(&oid, &name); scanErr != nil {
			continue
		}
		registerDateTimeOID(tm, oid, name)
	}
	return nil
}

func registerDateTimeOID(tm *pgtype.Map, oid uint32, name string) {
	if tm == nil || oid == 0 {
		return
	}
	n := strings.ToLower(strings.TrimSpace(name))
	var codec pgtype.Codec
	switch {
	case strings.Contains(n, "timestamptz"), strings.Contains(n, "with time zone") && strings.Contains(n, "timestamp"):
		codec = pgtype.TimestamptzCodec{}
	case n == "date":
		codec = pgtype.DateCodec{}
	case strings.HasPrefix(n, "timetz"), n == "time with time zone":
		codec = pgtype.TimeCodec{}
	case strings.HasPrefix(n, "time"):
		// time / timestamp / datetime / smalldatetime
		if strings.Contains(n, "stamp") || n == "datetime" || n == "smalldatetime" {
			codec = pgtype.TimestampCodec{}
		} else {
			codec = pgtype.TimeCodec{}
		}
	default:
		codec = pgtype.TimestampCodec{}
	}
	tm.RegisterType(&pgtype.Type{
		Name:  name,
		OID:   oid,
		Codec: codec,
	})
}

type compatiblePostgresDialector struct {
	*postgres.Dialector
}

func (d compatiblePostgresDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return compatiblePostgresMigrator{
		Migrator: d.Dialector.Migrator(db).(postgres.Migrator),
	}
}

type compatiblePostgresMigrator struct {
	postgres.Migrator
}

func (m compatiblePostgresMigrator) ColumnTypes(value interface{}) ([]gorm.ColumnType, error) {
	columnTypes, err := m.Migrator.ColumnTypes(value)
	if err == nil {
		return columnTypes, nil
	}
	if !isKingbaseUdtNameError(err) {
		return nil, err
	}
	return m.columnTypesFallback(value)
}

// columnTypesFallback 不依赖 udt_name / pg_type，仅用 information_schema.columns.data_type。
func (m compatiblePostgresMigrator) columnTypesFallback(value interface{}) (columnTypes []gorm.ColumnType, err error) {
	columnTypes = make([]gorm.ColumnType, 0)
	err = m.RunWithValue(value, func(stmt *gorm.Statement) error {
		currentDatabase := m.DB.Migrator().CurrentDatabase()
		currentSchema, table := m.CurrentSchema(stmt, stmt.Table)
		rows, queryErr := m.DB.Raw(
			`SELECT c.column_name,
				c.is_nullable = 'YES',
				c.data_type,
				c.character_maximum_length,
				c.numeric_precision,
				c.numeric_precision_radix,
				c.numeric_scale,
				c.datetime_precision,
				c.column_default
			FROM information_schema.columns AS c
			WHERE c.table_catalog = ? AND c.table_schema = ? AND c.table_name = ?`,
			currentDatabase, currentSchema, table,
		).Rows()
		if queryErr != nil {
			return queryErr
		}
		defer rows.Close()

		for rows.Next() {
			column := &gormmigrator.ColumnType{
				PrimaryKeyValue: sql.NullBool{Valid: true},
				UniqueValue:     sql.NullBool{Valid: true},
			}
			var (
				datetimePrecision sql.NullInt64
				radixValue        sql.NullInt64
			)
			if scanErr := rows.Scan(
				&column.NameValue,
				&column.NullableValue,
				&column.DataTypeValue,
				&column.LengthValue,
				&column.DecimalSizeValue,
				&radixValue,
				&column.ScaleValue,
				&datetimePrecision,
				&column.DefaultValueValue,
			); scanErr != nil {
				return scanErr
			}

			if column.DefaultValueValue.Valid {
				def := column.DefaultValueValue.String
				if strings.HasPrefix(def, "nextval('") && strings.HasSuffix(def, "seq'::regclass)") {
					column.AutoIncrementValue = sql.NullBool{Bool: true, Valid: true}
					column.DefaultValueValue = sql.NullString{}
				} else {
					column.DefaultValueValue.String = regexp.MustCompile(`'?(.*)\b'?:+[\w\s]+$`).ReplaceAllString(def, "$1")
				}
			}
			if datetimePrecision.Valid {
				column.DecimalSizeValue = datetimePrecision
			}
			column.DataTypeValue.String = normalizeSQLDataType(column.DataTypeValue.String)
			columnTypes = append(columnTypes, column)
		}
		return rows.Err()
	})
	return
}

func normalizeSQLDataType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "character varying", "varchar":
		return "varchar"
	case "character", "char":
		return "char"
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "time without time zone":
		return "time"
	case "double precision":
		return "float8"
	case "real":
		return "float4"
	case "integer":
		return "int4"
	case "bigint":
		return "int8"
	case "smallint":
		return "int2"
	case "boolean":
		return "bool"
	default:
		return t
	}
}
