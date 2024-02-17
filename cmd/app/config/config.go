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

type Config struct {
	Default DefaultOptions `yaml:"default"`
	Mysql   MysqlOptions   `yaml:"mysql"`
}

type DefaultOptions struct {
	Mode   string `yaml:"mode"`
	Listen int    `yaml:"listen"`
	JWTKey string `yaml:"jwt_key"`

	// 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
	AutoMigrate bool `yaml:"auto_migrate"`
}

// MysqlOptions 数据库具体配置
type MysqlOptions struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
}

func (c *Config) Valid() error {
	return nil
}
