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

package options

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/pkg/db"
	pixiuModel "github.com/caoyingjunz/pixiu/pkg/db/model"
	pixiuutil "github.com/caoyingjunz/pixiu/pkg/util"
	"k8s.io/klog/v2"
)

const (
	RunnerAgentV2 = "runner-agent-v2"
	RunnerAgentV3 = "runner-agent-v3"

	RunnerAgentV2Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2"
	RunnerAgentV3Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.4"
)

const (
	distributionFamilyCentos    = "CentOS"
	distributionFamilyUbuntu    = "Ubuntu"
	distributionFamilyDebian    = "Debian"
	distributionFamilyOpenEuler = "OpenEuler"
	distributionFamilyRocky     = "RockyLinux"
	distributionFamilyKylin     = "Kylin"
)

var defaultDistributionCatalog = []struct {
	family string
	name   string
	runner string
}{
	{
		family: distributionFamilyCentos,
		name:   "centos7",
		runner: RunnerAgentV2,
	},
	{
		family: distributionFamilyUbuntu,
		name:   "ubuntu18.04",
		runner: RunnerAgentV2,
	},
	{
		family: distributionFamilyUbuntu,
		name:   "ubuntu20.04",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyUbuntu,
		name:   "ubuntu22.04",
		runner: RunnerAgentV3,
	}, {
		family: distributionFamilyUbuntu,
		name:   "ubuntu24.04",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyDebian,
		name:   "debian10",
		runner: RunnerAgentV2,
	},
	{
		family: distributionFamilyDebian,
		name:   "debian11",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyOpenEuler,
		name:   "openEuler22.03",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyOpenEuler,
		name:   "openEuler24.03",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyRocky,
		name:   "rocky8.5",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyRocky,
		name:   "rocky9.2",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyRocky,
		name:   "rocky9.3",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyRocky,
		name:   "rocky10.2",
		runner: RunnerAgentV3,
	},
	{
		family: distributionFamilyKylin,
		name:   "kylinV10",
		runner: RunnerAgentV3,
	},
}

var defaultAIProviderCatalog = []pixiuModel.AIProvider{
	{
		Name:        "openai",
		BaseURL:     "https://api.openai.com",
		Protocol:    "openai_responses",
		Description: "OpenAI official API",
		MaxTokens:   4096,
		Builtin:     true,
	},
	{
		Name:        "deepseek",
		BaseURL:     "https://api.deepseek.com",
		Protocol:    "openai_chat",
		Description: "DeepSeek official API",
		MaxTokens:   4096,
		Builtin:     true,
	},
	{
		Name:        "siliconflow",
		BaseURL:     "https://api.siliconflow.cn/v1",
		Protocol:    "openai_chat",
		Description: "SiliconFlow official API",
		MaxTokens:   4096,
		Builtin:     true,
	},
	{
		Name:        "zhipu",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		Protocol:    "openai_chat",
		Description: "Zhipu GLM official API",
		MaxTokens:   4096,
		Builtin:     true,
	},
}

var defaultRunners = []struct {
	name        string
	engineImage string
	desc        string
}{
	{
		name:        RunnerAgentV2,
		engineImage: RunnerAgentV2Image,
		desc:        "操作系统默认 python2",
	},
	{
		name:        RunnerAgentV3,
		engineImage: RunnerAgentV3Image,
		desc:        "操作系统默认 python3",
	},
}

// bootstrapDatabase 启动时集中初始化所有数据库相关资源
func (o *Options) bootstrapDatabase() error {
	ctx := context.Background()

	// API 资源要等路由安装后才会入库；此处只初始化内置只读租户和角色。
	if err := o.bootstrapBuiltinReadonlyResources(ctx); err != nil {
		return err
	}
	// 初始化超级管理员
	if err := o.bootstrapRootUser(ctx); err != nil {
		return err
	}
	if err := o.bootstrapAIProviders(ctx); err != nil {
		return err
	}
	// 初始化操作系统
	if err := o.bootstrapDistributions(ctx); err != nil {
		return err
	}
	// 初始化 Runner
	if err := o.bootstrapRunners(ctx); err != nil {
		return err
	}
	return nil
}

func (o *Options) bootstrapBuiltinReadonlyResources(ctx context.Context) error {
	tenant := &pixiuModel.Tenant{
		Name:        pixiuModel.DefaultTenantName,
		Description: "内置普通用户使用的租户",
	}
	if err := o.Factory.Tenant().EnsureBuiltin(ctx, tenant); err != nil {
		return fmt.Errorf("failed to initialize builtin readonly tenant: %v", err)
	}

	role := &pixiuModel.Role{
		TenantId:    tenant.Id,
		Name:        pixiuModel.BuiltinReadonlyRoleName,
		Description: "内置普通用户使用的只读角色",
	}
	if err := o.Factory.Role().EnsureBuiltin(ctx, role); err != nil {
		return fmt.Errorf("failed to initialize builtin readonly role: %v", err)
	}
	return nil
}

type readonlyAPI struct {
	method string
	path   string
}

var builtinReadonlyAPIs = []readonlyAPI{
	{method: "GET", path: "/pixiu/clusters"},
	{method: "GET", path: "/pixiu/clusters/:clusterId"},
	{method: "GET", path: "/pixiu/clusters/permissions/:permissionId"},
	{method: "GET", path: "/pixiu/datasources"},
	{method: "GET", path: "/pixiu/datasources/:datasourceId"},
	{method: "GET", path: "/pixiu/alerts/rules"},
	{method: "GET", path: "/pixiu/alerts/rules/:ruleId"},
	{method: "GET", path: "/pixiu/alerts/events"},
	{method: "GET", path: "/pixiu/alerts/events/:eventId"},
	{method: "GET", path: "/pixiu/alerts/channels"},
	{method: "GET", path: "/pixiu/alerts/channels/:channelId"},
	{method: "GET", path: "/pixiu/alerts/notifications"},
	{method: "GET", path: "/pixiu/alerts/silences"},
	{method: "GET", path: "/pixiu/alerts/silences/:silenceId"},
	{method: "GET", path: "/pixiu/kubeproxy/clusters/:cluster/namespaces/:namespace/pods/:pod/log"},
	{method: "GET", path: "/pixiu/kubeproxy/clusters/:cluster/namespaces/:namespace/name/:name/kind/:kind/events"},
	{method: "GET", path: "/pixiu/kubeproxy/clusters/:cluster/namespaces/:namespace/pods/:pod/files"},
	{method: "GET", path: "/pixiu/kubeproxy/clusters/:cluster/namespaces/:namespace/pods/:pod/files/download"},
}

var builtinReadonlyMenus = []string{
	"container.cluster",
	"monitor.realtime",
	"monitor.logs",
	"monitor.alert",
	"monitor.datasource",
	"system.user-center",
}

// SyncBuiltinReadonlyRolePermissions runs after route installation, when all persisted APIs exist.
func (o *Options) SyncBuiltinReadonlyRolePermissions(ctx context.Context) error {
	tenant, err := o.Factory.Tenant().GetTenantByName(ctx, pixiuModel.DefaultTenantName)
	if err != nil || tenant == nil {
		return fmt.Errorf("failed to resolve builtin readonly tenant: %v", err)
	}
	role, err := o.Factory.Role().GetBy(ctx, db.WithTenantId(tenant.Id), db.WithName(pixiuModel.BuiltinReadonlyRoleName))
	if err != nil || role == nil {
		return fmt.Errorf("failed to resolve builtin readonly role: %v", err)
	}

	apiIDs := make([]int64, 0, len(builtinReadonlyAPIs))
	for _, endpoint := range builtinReadonlyAPIs {
		api, getErr := o.Factory.API().GetByMethodAndPath(ctx, endpoint.method, endpoint.path)
		if getErr != nil || api == nil {
			return fmt.Errorf("failed to resolve builtin readonly api %s %s: %v", endpoint.method, endpoint.path, getErr)
		}
		apiIDs = append(apiIDs, api.Id)
	}
	if err = o.Factory.Role().API().ReplaceByRoleId(ctx, role.Id, apiIDs); err != nil {
		return fmt.Errorf("failed to sync builtin readonly apis: %v", err)
	}
	if err = o.Factory.Role().Menu().ReplaceByRoleId(ctx, role.Id, builtinReadonlyMenus); err != nil {
		return fmt.Errorf("failed to sync builtin readonly menus: %v", err)
	}
	return nil
}

func (o *Options) bootstrapAIProviders(ctx context.Context) error {
	for i := range defaultAIProviderCatalog {
		provider := defaultAIProviderCatalog[i]
		if err := o.Factory.Assistant().Provider().EnsureBuiltin(ctx, &provider); err != nil {
			return fmt.Errorf("failed to initialize ai provider %s: %v", provider.Name, err)
		}
	}
	return nil
}

// bootstrapRootUser 启动时自动初始化超级管理员账户
// 若超管已存在则跳过，若不存在则使用配置文件中的用户名和密码创建
// 密码经由 pixiuutil.EncryptUserPassword() bcrypt 加密后直接经 factory 入库
// （不经过 Controller.User().Create：其首行 CheckRoot 依赖请求上下文，启动阶段无 user 会报错）
func (o *Options) bootstrapRootUser(ctx context.Context) error {
	root, err := o.Factory.User().GetRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to check root user: %v", err)
	}
	if root != nil {
		klog.Info("root user already exists, skipping")
		return nil
	}

	adminUser := o.ComponentConfig.Default.AdminUser
	adminPassword := o.ComponentConfig.Default.AdminPassword
	klog.Infof("initializing root user: %s", adminUser)

	encrypted, err := pixiuutil.EncryptUserPassword(adminPassword)
	if err != nil {
		return fmt.Errorf("failed to encrypt admin password: %v", err)
	}
	if _, err = o.Factory.User().Create(ctx, &pixiuModel.User{
		Name:     adminUser,
		Password: encrypted,
		Status:   pixiuModel.UserStatusNormal,
		Role:     pixiuModel.RoleRoot,
	}); err != nil {
		return fmt.Errorf("failed to create root user: %v", err)
	}
	return nil
}

func (o *Options) bootstrapDistributions(ctx context.Context) error {
	existsDistros, err := o.Factory.Distribution().ListDistributions(ctx)
	if err != nil {
		klog.Errorf("failed to list runners: %v", err)
		return err
	}

	// 构建已存在 Distro 的 map，用于快速查找
	existingDistroMap := make(map[string]bool)
	for _, d := range existsDistros {
		existingDistroMap[d.Name] = true
	}

	for _, distro := range defaultDistributionCatalog {
		if existingDistroMap[distro.name] {
			continue
		}

		object := &pixiuModel.Distribution{
			Family: distro.family,
			Name:   distro.name,
			Runner: distro.runner,
		}
		if _, err = o.Factory.Distribution().CreateDistribution(ctx, object); err != nil {
			continue
		}
	}

	klog.Infof("operating system initialization completed")
	return nil
}

// bootstrapRunners 启动时自动初始化默认 Runner
func (o *Options) bootstrapRunners(ctx context.Context) error {
	// 先获取所有已存在的 runners
	existingRunners, err := o.Factory.Runner().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list runners: %v", err)
	}

	// 构建已存在 runner 的 map，用于快速查找
	existingRunnerMap := make(map[string]bool)
	for _, r := range existingRunners {
		existingRunnerMap[r.Name] = true
	}

	for _, dr := range defaultRunners {
		if existingRunnerMap[dr.name] {
			klog.V(1).Infof("runner %s already exists, skipping", dr.name)
			continue
		}

		// 直接经 factory 入库，不经过 Controller.Runner().Create（其 CheckRoot 依赖请求上下文，启动阶段无 user 会报错）
		if _, err = o.Factory.Runner().Create(ctx, &pixiuModel.Runner{
			Name:        dr.name,
			EngineImage: dr.engineImage,
			Status:      pixiuModel.RunnerStatusUnstart,
			Description: dr.desc,
		}); err != nil {
			return fmt.Errorf("failed to bootstrap runner %s: %v", dr.name, err)
		}
	}

	klog.Infof("runner initialization completed")
	return nil
}
