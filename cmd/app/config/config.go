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
	"strings"

	"github.com/caoyingjunz/gopixiu/pkg/types"
)

type Config struct {
	Default DefaultOptions `yaml:"default"`
	Mysql   MysqlOptions   `yaml:"mysql"`
	Cicd    CicdOptions    `yaml:"cicd"`
}

type DefaultOptions struct {
	Listen   int    `yaml:"listen"`
	LogType  string `yaml:"log_type"`
	LogDir   string `yaml:"log_dir"`
	LogLevel string `yaml:"log_level"`
	JWTKey   string `yaml:"jwt_key"`
}

type MysqlOptions struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
}

type CicdOptions struct {
	Driver  string          `yaml:"driver"`
	Jenkins *JenkinsOptions `yaml:"jenkins"`
}

type JenkinsOptions struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

func (c *Config) Valid() error {
	if strings.ToLower(c.Default.LogType) == "file" {
		if len(c.Default.LogDir) == 0 {
			return fmt.Errorf("log_dir should be config when log type is file")
		}
	}

	switch c.Cicd.Driver {
	case "", types.Jenkins:
		j := c.Cicd.Jenkins
		if j == nil {
			return fmt.Errorf("jenkins config option missing")
		}
	default:
		return fmt.Errorf("unsupported cicd type %s", c.Cicd.Driver)
	}

	return nil
}
