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
	"github.com/caoyingjunz/pixiu/pkg/jobmanager"
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
	Default     DefaultOptions          `yaml:"default"`
	Database    DatabaseOptions         `yaml:"database"`
	Worker      WorkerOptions           `yaml:"worker"`
	Audit       jobmanager.AuditOptions `yaml:"audit"`
	Log         LogOptions              `yaml:"log"`
	TLS         TLSOptions              `yaml:"tls"`
	KubeGateway KubeGatewayOptions      `yaml:"kube_gateway"`

	AlertHistory jobmanager.AlertHistoryOptions `yaml:"alert"`
}

// TLSOptions HTTPS 监听配置。kubectl 经 /k8s 网关必须走 HTTPS（client-go 不会在明文 HTTP 上发送 Bearer token）。
type TLSOptions struct {
	Enable   bool   `yaml:"enable"`
	Listen   int    `yaml:"listen"` // HTTPS 端口，默认 8443
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func (o *TLSOptions) SetDefaults() {
	if o == nil {
		return
	}
	if o.Listen <= 0 {
		o.Listen = 8443
	}
}

func (o *TLSOptions) IsEnabled() bool {
	return o != nil && o.Enable
}

type DefaultOptions struct {
	Mode   Mode   `yaml:"mode"`
	Listen int    `yaml:"listen"`
	JWTKey string `yaml:"jwt_key"`
	// CloudShell/工具容器镜像
	Toolbox string `yaml:"toolbox"`

	// 自动创建指定模型的数据库表结构，不会更新已存在的数据库表
	AutoMigrate bool `yaml:"auto_migrate"`

	// 静态文件路径
	StaticFiles string `yaml:"static_files"`

	// 对外访问地址（Agent 反向隧道使用），例如 https://pixiu.example.com
	PublicURL string `yaml:"public_url"`

	// 超级管理员初始化配置，留空则使用默认值
	AdminUser     string `yaml:"admin_user"`
	AdminPassword string `yaml:"admin_password"`

	// 启用单人登录限制
	// true: 同账号仅允许单人在线；false: 允许多人同时在线
	// 默认值为 false
	SingleLogin bool `yaml:"single_login"`
}

func (o DefaultOptions) Valid() error {
	return nil
}

// KubeGatewayOptions 集群代理 kubeconfig /k8s 网关配置。
type KubeGatewayOptions struct {
	Enabled               *bool `yaml:"enabled"`
	DefaultExpireHours    int   `yaml:"default_expire_hours"`
	MaxExpireHours        int   `yaml:"max_expire_hours"`
	InsecureSkipTLSVerify bool  `yaml:"insecure_skip_tls_verify"`
}

func (o *KubeGatewayOptions) IsEnabled() bool {
	if o == nil || o.Enabled == nil {
		return true
	}
	return *o.Enabled
}

// SetDefaults 补齐未配置或非法的过期时间默认值。
func (o *KubeGatewayOptions) SetDefaults() {
	if o.DefaultExpireHours <= 0 {
		o.DefaultExpireHours = 720
	}
	if o.MaxExpireHours <= 0 {
		o.MaxExpireHours = 8760
	}
	if o.DefaultExpireHours > o.MaxExpireHours {
		o.DefaultExpireHours = o.MaxExpireHours
	}
}

func (o KubeGatewayOptions) Valid() error {
	return nil
}

// DatabaseOptions 数据库具体配置
type DatabaseOptions struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
}

func (o DatabaseOptions) Valid() error {
	// TODO
	return nil
}

type WorkerOptions struct {
	WorkDir string   `yaml:"work_dir"`
	Engines []Engine `yaml:"engines"`
}

type Engine struct {
	Name        string   `yaml:"name"`
	Image       string   `yaml:"image"`
	OSSupported []string `yaml:"os_supported"`
}

func (w WorkerOptions) Valid() error {
	// TODO
	return nil
}

func (c *Config) Valid() (err error) {
	if err = c.Default.Valid(); err != nil {
		return
	}
	if err = c.Log.Valid(); err != nil {
		return
	}
	if err = c.Database.Valid(); err != nil {
		return
	}
	if err = c.Worker.Valid(); err != nil {
		return
	}
	c.KubeGateway.SetDefaults()
	if err = c.KubeGateway.Valid(); err != nil {
		return
	}
	c.TLS.SetDefaults()

	return
}
