package options

import (
	"context"
	"fmt"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	pixiuModel "github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"k8s.io/klog/v2"
)

const (
	RunnerAgentV2 = "runner-agent-v2"
	RunnerAgentV3 = "runner-agent-v3"

	RunnerAgentV2Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v2.0.2"
	RunnerAgentV3Image = "ccr.ccs.tencentyun.com/pixiucloud/kubez-ansible:v3.0.2"
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
	names  []string
}{
	{distributionFamilyCentos, []string{"centos7"}},
	{distributionFamilyUbuntu, []string{"ubuntu18.04", "ubuntu20.04", "ubuntu22.04"}},
	{distributionFamilyDebian, []string{"debian10", "debian11"}},
	{distributionFamilyOpenEuler, []string{"openEuler22.03", "openEuler24.03"}},
	{distributionFamilyRocky, []string{"rocky8.5", "rocky9.2", "rocky9.3"}},
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

func runnerByName(cfg config.Config) map[string]string {
	engines := cfg.Worker.Engines
	if len(engines) == 0 {
		engines = config.DefaultEngines()
	}

	runnerByOSName := make(map[string]string, len(engines)*4)
	for _, engine := range engines {
		for _, name := range engine.OSSupported {
			runnerByOSName[name] = engine.Name
		}
	}
	return runnerByOSName
}

func (o *Options) bootstrapDistributions(ctx context.Context) error {
	runnerMap := runnerByName(o.ComponentConfig)
	klog.Infof("bootstrapping distributions")

	for _, item := range defaultDistributionCatalog {
		for _, name := range item.names {
			runner := runnerMap[name]
			if runner == "" {
				klog.Warningf("no runner configured for distribution name %s", name)
				continue
			}

			// 检查是否已存在
			existing, err := o.Factory.Distribution().GetDistributionByFamilyName(ctx, item.family, name)
			if err != nil {
				return fmt.Errorf("failed to check distribution %s/%s: %w", item.family, name, err)
			}
			if existing != nil {
				klog.V(1).Infof("distribution %s/%s already exists, skipping", item.family, name)
				continue
			}

			object := &pixiuModel.Distribution{
				Family: item.family,
				Name:   name,
				Runner: runner,
			}
			if _, err = o.Factory.Distribution().CreateDistribution(ctx, object); err != nil {
				continue
			}
		}
	}
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

		if err := o.Controller.Runner().Create(ctx, &types.CreateRunnerRequest{
			Name:        dr.name,
			EngineImage: dr.engineImage,
			Status:      pixiuModel.RunnerStatusUnknown,
			Description: dr.desc,
		}); err != nil {
			return fmt.Errorf("failed to bootstrap runner %s: %v", dr.name, err)
		}
	}

	klog.Infof("完成 runner agent 的初始化")
	return nil
}
