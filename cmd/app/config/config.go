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
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/jobmanager"
	logutil "github.com/caoyingjunz/pixiu/pkg/util/log"
)

type Mode string

const (
	DebugMode   Mode = "debug"
	ReleaseMode Mode = "release"
)

func (m Mode) InDebug() bool {
	return m == DebugMode
}

type Config struct {
	Default DefaultOptions          `yaml:"default"`
	Mysql   MysqlOptions            `yaml:"mysql"`
	Worker  WorkerOptions           `yaml:"worker"`
	Audit   jobmanager.AuditOptions `yaml:"audit"`
	TLS     *TLS                    `yaml:"tls"`
}

type DefaultOptions struct {
	Mode   Mode   `yaml:"mode"`
	Listen int    `yaml:"listen"`
	JWTKey string `yaml:"jwt_key"`

	// 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
	AutoMigrate bool `yaml:"auto_migrate"`

	logutil.LogOptions `yaml:",inline"`
	// 静态文件路径
	StaticFiles string `yaml:"static_files"`
}

func (o DefaultOptions) Valid() error {
	if err := o.LogOptions.Valid(); err != nil {
		return err
	}
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

func (o MysqlOptions) Valid() error {
	// TODO
	return nil
}

type WorkerOptions struct {
	WorkDir string   `yaml:"work_dir"`
	Engines []Engine `yaml:"engines"`
}

type Engine struct {
	Image       string   `yaml:"image"`
	OSSupported []string `yaml:"os_supported"`
}

func (w WorkerOptions) Valid() error {
	// TODO
	return nil
}

type TLS struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func (t *TLS) Valid() error {
	if t != nil {
		if len(t.CertFile) == 0 {
			return fmt.Errorf("listen on tls, no cert_file found")
		}

		if len(t.KeyFile) == 0 {
			return fmt.Errorf("listen on tls, no key_file found")
		}
	}

	return nil
}

func (c *Config) Valid() (err error) {
	if err = c.Default.Valid(); err != nil {
		return
	}
	if err = c.Mysql.Valid(); err != nil {
		return
	}
	if err = c.Worker.Valid(); err != nil {
		return
	}
	if err = c.TLS.Valid(); err != nil {
		return err
	}

	return
}
