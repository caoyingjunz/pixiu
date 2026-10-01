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
	"path/filepath"
	"strings"

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
	Mysql       MysqlOptions            `yaml:"mysql"`
	Worker      WorkerOptions           `yaml:"worker"`
	Audit       jobmanager.AuditOptions `yaml:"audit"`
	Log         LogOptions              `yaml:"log"`
	TLS         TLSOptions              `yaml:"tls"`
	KubeGateway KubeGatewayOptions      `yaml:"kube_gateway"`
	Runtime     RuntimeOptions          `yaml:"runtime"`

	AlertHistory jobmanager.AlertHistoryOptions `yaml:"alert"`
	// CronHpaHistory 定时扩缩容执行历史清理配置
	CronHpaHistory jobmanager.CronHpaHistoryOptions `yaml:"cron_hpa"`
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
	// DeployTimeout 本地部署任务容器等待超时秒数，默认 900 秒（等待循环每 5 秒轮询一次容器状态）
	DeployTimeout int `yaml:"deploy_timeout"`
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

// RuntimeOptions 宿主容器运行时配置（pixiu 调用本机运行时拉起 runner 容器）。
type RuntimeOptions struct {
	CRI string `yaml:"cri"` // docker | containerd，默认 containerd
	// Socket 宿主运行时 socket 文件路径（裸路径，如 /run/containerd/containerd.sock）。
	// 留空时：containerd 使用 /run/containerd/containerd.sock；docker 沿用 DOCKER_HOST/默认 socket。
	Socket     string                   `yaml:"socket"`
	Containerd ContainerdRuntimeOptions `yaml:"containerd"`
}

type ContainerdRuntimeOptions struct {
	Namespace string `yaml:"namespace"` // 默认 default；禁止 k8s.io
}

// defaultContainerdSocket containerd 的默认 socket 路径（runtime.socket 留空且 CRI 为 containerd 时使用）
const defaultContainerdSocket = "/run/containerd/containerd.sock"

func (o *RuntimeOptions) SetDefaults() {
	if o.CRI == "" {
		o.CRI = "containerd"
	}
	// socket 默认值仅 containerd 有固定路径；docker 留空表示沿用 DOCKER_HOST/默认 socket
	if o.Socket == "" && o.CRI == "containerd" {
		o.Socket = defaultContainerdSocket
	}
	if o.Containerd.Namespace == "" {
		o.Containerd.Namespace = "default"
	}
}

func (o RuntimeOptions) Valid() error {
	if o.CRI != "docker" && o.CRI != "containerd" {
		return fmt.Errorf("runtime.cri 取值非法(%s)，可选值: docker, containerd", o.CRI)
	}
	// k8s.io 命名空间是 kubelet 的工作区，在其中创建容器会污染 kubelet 视图，禁止使用
	if o.CRI == "containerd" && o.Containerd.Namespace == "k8s.io" {
		return fmt.Errorf("runtime.containerd.namespace 禁止使用 k8s.io，会污染 kubelet 视图")
	}
	if o.Socket != "" {
		if strings.Contains(o.Socket, "://") {
			return fmt.Errorf("runtime.socket 只支持裸 socket 路径(%s)，请去掉 unix:// 等协议前缀", o.Socket)
		}
		if !filepath.IsAbs(o.Socket) {
			return fmt.Errorf("runtime.socket 必须是绝对路径(%s)", o.Socket)
		}
	}

	return nil
}

func (c *Config) Valid() (err error) {
	if err = c.Default.Valid(); err != nil {
		return
	}
	if err = c.Log.Valid(); err != nil {
		return
	}
	if err = c.Mysql.Valid(); err != nil {
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
	c.Runtime.SetDefaults()
	if err = c.Runtime.Valid(); err != nil {
		return
	}

	return
}
