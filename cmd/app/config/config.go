/*
Copyright 2021 The Pixiu Authors.

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

package config

import (
	"errors"
)

type LogFormat string

const (
	LogFormatJson LogFormat = "json"
	LogFormatText LogFormat = "text"
)

var ErrInvalidLogFormat = errors.New("invalid log format")

type Config struct {
	Default DefaultOptions `yaml:"default"`
	Db      DbConfig       `yaml:"db"`
}

type DefaultOptions struct {
	Mode   string `yaml:"mode"`
	Listen int    `yaml:"listen"`
	JWTKey string `yaml:"jwt_key"`

	// 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
	AutoMigrate bool `yaml:"auto_migrate"`

	LogOptions `yaml:",inline"`
}

type DbConfig struct {
	Sqlite *SqliteOptions `yaml:"sqlite"`
	Mysql  *MysqlOptions  `yaml:"mysql"`
}

func (d DbConfig) Valid() error {
	if d.Sqlite != nil {
		return d.Sqlite.Valid()
	}
	if d.Mysql != nil {
		return d.Mysql.Valid()
	}
	return errors.New("invalid database config")
}

type SqliteOptions struct {
	Db string `yaml:"db"`
}

func (o *SqliteOptions) Valid() error {
	// TODO
	return nil
}

// MysqlOptions 数据库具体配置
type MysqlOptions struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
}

func (o *MysqlOptions) Valid() error {
	// TODO
	return nil
}

type LogOptions struct {
	LogFormat `yaml:"log_format"`
}

func (o LogOptions) Valid() error {
	switch o.LogFormat {
	case LogFormatJson, LogFormatText:
		return nil
	default:
		return ErrInvalidLogFormat
	}
}

func (c *Config) Valid() (err error) {
	if err = c.Default.Valid(); err != nil {
		return
	}
	if err = c.Db.Valid(); err != nil {
		return
	}
	return
}
