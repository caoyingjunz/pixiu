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
	"time"

	"gorm.io/gorm/logger"
)

type (
	SQLs []string

	DBLogger struct {
		logger.LogLevel
		SlowThreshold time.Duration // slow SQL queries
	}
)

const SQLContextKey = "sqls"

func NewLogger(level logger.LogLevel, slowThreshold time.Duration) *DBLogger {
	return &DBLogger{
		LogLevel:      level,
		SlowThreshold: slowThreshold,
	}
}

func (l *DBLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.LogLevel = level
	return l
}

func (l *DBLogger) Info(ctx context.Context, msg string, data ...interface{}) {}

func (l *DBLogger) Warn(ctx context.Context, msg string, data ...interface{}) {}

func (l *DBLogger) Error(ctx context.Context, msg string, data ...interface{}) {}

func (l *DBLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	sql, _ := fc()
	if v := ctx.Value(SQLContextKey); v != nil {
		sqls := v.(*SQLs)
		*sqls = append(*sqls, sql)
	}
}

// GetSQLs returns all the SQL statements executed in the current context.
func GetSQLs(ctx context.Context) SQLs {
	if v := ctx.Value(SQLContextKey); v != nil {
		return *v.(*SQLs)
	}
	return SQLs{}
}
