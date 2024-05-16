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
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

const (
	MaxIdleConns = 10
	MaxOpenConns = 100
)

// DB is the interface for database operations.
type DB interface {
	// Open connects to database.
	Open() error
	// Close closes the database connection.
	Close() error

	ShareDaoFactory
}

func NewDB(cfg config.DbConfig, debug bool) DB {
	if cfg.Mysql != nil {
		return NewMySQLStore(cfg.Mysql, debug)
	}
	if cfg.Sqlite != nil {
		return NewSQLiteStore(cfg.Sqlite, debug)
	}

	return nil
}

var _ DB = (*mysqlStore)(nil)
var _ ShareDaoFactory = (*mysqlStore)(nil)

// MySQL implementation
type mysqlStore struct {
	cfg       *config.MysqlOptions
	db        *gorm.DB
	debugMode bool
}

func NewMySQLStore(cfg *config.MysqlOptions, debug bool) *mysqlStore {
	return &mysqlStore{
		cfg:       cfg,
		debugMode: debug,
	}
}

// getDSN returns the DSN string for MySQL.
func (s *mysqlStore) getDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		s.cfg.User,
		s.cfg.Password,
		s.cfg.Host,
		s.cfg.Port,
		s.cfg.Name)
}

// Open connects to MySQL.
func (s *mysqlStore) Open() (err error) {
	opt := &gorm.Config{}
	if s.debugMode {
		opt.Logger = logger.Default.LogMode(logger.Info)
	}
	if s.db, err = gorm.Open(mysql.Open(s.getDSN()), opt); err != nil {
		return
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return
	}
	sqlDB.SetMaxIdleConns(MaxIdleConns)
	sqlDB.SetMaxOpenConns(MaxOpenConns)

	return sqlDB.Ping()
}

// Close closes the MySQL connection.
func (s *mysqlStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate applies the latest data models to MySQL.
func (s *mysqlStore) Migrate() error {
	return newMigrator(s.db).createTables(model.GetMigrationModels()...)
}

// Cluster implements the ClusterInterface with MySQL.
func (s *mysqlStore) Cluster() ClusterInterface {
	return newClusterMySQL(s.db)
}

// User implements the UserInterface with MySQL.
func (s *mysqlStore) User() UserInterface {
	return newUserMySQL(s.db)
}

// Tenant implements the TenantInterface with MySQL.
func (s *mysqlStore) Tenant() TenantInterface {
	return newTenantMySQL(s.db)
}

var _ DB = (*sqliteStore)(nil)
var _ ShareDaoFactory = (*sqliteStore)(nil)

// SQLite implementation
type sqliteStore struct {
	cfg       *config.SqliteOptions
	db        *gorm.DB
	debugMode bool
}

func NewSQLiteStore(cfg *config.SqliteOptions, debug bool) *sqliteStore {
	return &sqliteStore{
		cfg:       cfg,
		debugMode: debug,
	}
}

// getDSN returns the DSN string for SQLite.
func (s *sqliteStore) getDSN() string {
	return fmt.Sprintf("%s?charset=utf8&parseTime=True&loc=Local", s.cfg.Db)
}

// Open connects to SQLite.
func (s *sqliteStore) Open() (err error) {
	opt := &gorm.Config{}
	if s.debugMode {
		opt.Logger = logger.Default.LogMode(logger.Info)
	}
	if s.db, err = gorm.Open(sqlite.Open(s.getDSN()), opt); err != nil {
		return
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return
	}
	sqlDB.SetMaxIdleConns(MaxIdleConns)
	sqlDB.SetMaxOpenConns(MaxOpenConns)

	return sqlDB.Ping()
}

// Close closes the SQLite connection.
func (s *sqliteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate applies the latest data models to SQLite.
func (s *sqliteStore) Migrate() error {
	return newMigrator(s.db).createTables(model.GetMigrationModels()...)
}

// Cluster implements the ClusterInterface with SQLite.
func (s *sqliteStore) Cluster() ClusterInterface {
	return newClusterSQLite(s.db)
}

// User implements the UserInterface with SQLite.
func (s *sqliteStore) User() UserInterface {
	return newUserSQLite(s.db)
}

// Tenant implements the TenantInterface with SQLite.
func (s *sqliteStore) Tenant() TenantInterface {
	return newTenantSQLite(s.db)
}

// TODO: PostgreSQL implementation
// type pgStore struct {
// 	db *gorm.DB
// }
