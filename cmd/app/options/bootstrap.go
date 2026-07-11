package options

import (
	"context"
	"fmt"

	pixiuModel "github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"k8s.io/klog/v2"
)

const (
	RunnerAgentV2 = "runner-agent-v2"
	RunnerAgentV3 = "runner-agent-v3"

	RunnerAgentV2Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2"
	RunnerAgentV3Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.3"
)

const (
	distributionFamilyCentos    = "CentOS"
	distributionFamilyUbuntu    = "Ubuntu"
	distributionFamilyDebian    = "Debian"
	distributionFamilyOpenEuler = "OpenEuler"
	distributionFamilyRocky     = "RockyLinux"
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

	// 初始化超级管理员
	if err := o.bootstrapRootUser(ctx); err != nil {
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

// bootstrapRootUser 启动时自动初始化超级管理员账户
// 若超管已存在则跳过，若不存在则使用配置文件中的用户名和密码创建
// 密码经由 Controller.User().Create() 内部调用 util.EncryptUserPassword() 加密后入库
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

	return o.Controller.User().Create(ctx, &types.CreateUserRequest{
		Name:     adminUser,
		Password: adminPassword,
		Role:     pixiuModel.RoleRoot,
	})
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

	klog.Infof("完成支持系统的初始化")
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

		if err = o.Controller.Runner().Create(ctx, &types.CreateRunnerRequest{
			Name:        dr.name,
			EngineImage: dr.engineImage,
			Status:      pixiuModel.RunnerStatusUnstart,
			Description: dr.desc,
		}); err != nil {
			return fmt.Errorf("failed to bootstrap runner %s: %v", dr.name, err)
		}
	}

	klog.Infof("完成 runner 的初始化")
	return nil
}
