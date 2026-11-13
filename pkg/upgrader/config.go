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

package upgrader

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
)

const (
	defaultVersionURL    = "https://hub.pixiuio.com"
	defaultVersionRoute  = "/api/v2/pixiu/versions/latest"
	defaultContainerName = "pixiu"
	defaultInterval      = 10 * time.Second
	defaultPollInterval  = 5 * time.Second

	// 内置默认启动参数：用户不配置 run_args 时按运行时直接使用。
	defaultRunArgsContainerd = "--net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /run/containerd:/run/containerd -v /var/lib/containerd:/var/lib/containerd"
	defaultRunArgsDocker     = "--net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock"
)

// Config 是 pixiu-upgrade 的配置（与主服务共用 config.yaml，读取 upgrade/runtime/mysql 段），
// 即校验并补全默认值后的运行期配置。
type Config struct {
	Enabled         bool
	VersionURL      string
	VersionRoute    string
	ImageRepository string
	Interval        time.Duration
	ContainerName   string
	// UpgradeID 升级执行者标识（默认主机名），用于任务认领与状态回写校验。
	UpgradeID string
	// PollInterval 轮询数据库中升级请求的间隔。
	PollInterval time.Duration
	// RunArgs 重建 pixiu 容器时传给 nerdctl/docker run 的参数（不含 --name 与镜像）。
	// 内置按运行时的默认值，可不配置；需要额外挂载/参数时覆盖。
	RunArgs string

	Runtime config.RuntimeOptions
	Mysql   config.MysqlOptions
}

// fileConfig 是 config.yaml 的原始文件结构（与主服务共用，只取 upgrade/runtime/mysql 三段）。
type fileConfig struct {
	Upgrade upgradeSection        `yaml:"upgrade"`
	Runtime config.RuntimeOptions `yaml:"runtime"`
	Mysql   config.MysqlOptions   `yaml:"mysql"`
}

// upgradeSection 是 upgrade 配置段的原始形态（时长等字段为字符串，normalize 后转为 Config）。
type upgradeSection struct {
	// Enabled 为指针以区分「未配置」与「显式 false」：未配置时默认开启。
	Enabled         *bool  `yaml:"enabled"`
	VersionURL      string `yaml:"version_url"`
	VersionRoute    string `yaml:"version_route"`
	ImageRepository string `yaml:"image_repository"`
	Interval        string `yaml:"interval"`
	ContainerName   string `yaml:"container_name"`
	UpgradeID       string `yaml:"upgrade_id"`
	PollInterval    string `yaml:"poll_interval"`
	// RunArgs 通用兜底参数：对应运行时的 run_args_docker/run_args_containerd 为空时生效。
	RunArgs           string `yaml:"run_args"`
	RunArgsDocker     string `yaml:"run_args_docker"`
	RunArgsContainerd string `yaml:"run_args_containerd"`
}

// Load 读取并校验 pixiu 配置文件中的 upgrade/runtime/mysql 段（pixiu-upgrade 与主服务共用 config.yaml）。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置(%s)失败: %v", path, err)
	}

	var fileCfg fileConfig
	if err = yaml.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("解析配置(%s)失败: %v", path, err)
	}

	return normalize(fileCfg.Upgrade, fileCfg.Runtime, fileCfg.Mysql, true)
}

// FromUpgradeOptions 由主服务配置构造 upgrade 进程配置（版本检测在主服务内运行，不需要 mysql 段）。
func FromUpgradeOptions(opt config.UpgradeOptions, rt config.RuntimeOptions) (*Config, error) {
	enabled := opt.IsEnabled()
	return normalize(upgradeSection{
		Enabled:         &enabled,
		VersionURL:      opt.VersionURL,
		VersionRoute:    opt.VersionRoute,
		ImageRepository: opt.ImageRepository,
		Interval:        opt.Interval,
		ContainerName:   opt.ContainerName,
		PollInterval:    opt.PollInterval,
	}, rt, config.MysqlOptions{}, false)
}

// normalize 校验并补全默认值；requireMysql 为 true 时校验数据库配置（独立进程需要自行连库）。
func normalize(section upgradeSection, rt config.RuntimeOptions, mysql config.MysqlOptions, requireMysql bool) (*Config, error) {
	// 未配置（nil）时默认开启，仅显式 false 才关闭。
	enabled := section.Enabled == nil || *section.Enabled
	cfg := &Config{
		Enabled:         enabled,
		VersionURL:      section.VersionURL,
		ImageRepository: section.ImageRepository,
		ContainerName:   section.ContainerName,
		UpgradeID:       section.UpgradeID,
		Runtime:         rt,
		Mysql:           mysql,
	}

	// 启动参数解析顺序：
	// 1. 通用 run_args（trim）
	// 2. 按 CRI 覆盖（docker 取 run_args_docker，其余含未配置时取 run_args_containerd，非空才覆盖）
	// 3. 最终仍为空则使用对应 CRI 的内置默认值（未配置时 CRI 默认 containerd）
	runArgs := strings.TrimSpace(section.RunArgs)
	if rt.CRI == "docker" {
		if v := strings.TrimSpace(section.RunArgsDocker); v != "" {
			runArgs = v
		}
		if runArgs == "" {
			runArgs = defaultRunArgsDocker
		}
	} else {
		if v := strings.TrimSpace(section.RunArgsContainerd); v != "" {
			runArgs = v
		}
		if runArgs == "" {
			runArgs = defaultRunArgsContainerd
		}
	}
	cfg.RunArgs = runArgs

	if cfg.VersionURL == "" {
		cfg.VersionURL = defaultVersionURL
	}
	cfg.VersionRoute = defaultVersionRoute
	if section.VersionRoute != "" {
		cfg.VersionRoute = section.VersionRoute
	}
	cfg.VersionURL = strings.TrimRight(cfg.VersionURL, "/") + "/" + strings.TrimLeft(cfg.VersionRoute, "/")
	if cfg.ContainerName == "" {
		cfg.ContainerName = defaultContainerName
	}
	// 主服务检查器不传 upgrade_id，这里统一补默认值（主机名）
	if cfg.UpgradeID == "" {
		if hostname, hostErr := os.Hostname(); hostErr == nil && hostname != "" {
			cfg.UpgradeID = hostname
		} else {
			cfg.UpgradeID = "pixiu-upgrade"
		}
	}
	cfg.PollInterval = defaultPollInterval
	if section.PollInterval != "" {
		interval, parseErr := time.ParseDuration(section.PollInterval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.poll_interval 非法(%s): %v", section.PollInterval, parseErr)
		}
		cfg.PollInterval = interval
	}
	cfg.Interval = defaultInterval
	if section.Interval != "" {
		interval, parseErr := time.ParseDuration(section.Interval)
		if parseErr != nil {
			return nil, fmt.Errorf("upgrade.interval 非法(%s): %v", section.Interval, parseErr)
		}
		cfg.Interval = interval
	}
	if cfg.Interval < time.Second {
		return nil, fmt.Errorf("upgrade.interval 不能小于 1 秒")
	}
	if cfg.PollInterval < time.Second {
		return nil, fmt.Errorf("upgrade.poll_interval 不能小于 1 秒")
	}
	if requireMysql && (cfg.Mysql.Host == "" || cfg.Mysql.Name == "" || cfg.Mysql.User == "") {
		return nil, fmt.Errorf("mysql 配置不完整（host/name/user 必填）")
	}
	cfg.Runtime.SetDefaults()
	if err := cfg.Runtime.Valid(); err != nil {
		return nil, err
	}
	return cfg, nil
}
