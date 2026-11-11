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
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog/v2"

	upgradecontroller "github.com/caoyingjunz/pixiu/pkg/controller/upgrade"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

const (
	upgradeTimeout = 30 * time.Minute

	// startupConfirmTimeout/Interval 重建后确认容器保持运行的观察窗口
	startupConfirmTimeout  = 30 * time.Second
	startupConfirmInterval = 2 * time.Second
)

// Upgrade 定时检查外部版本并持久化到数据库；用户确认后由独立 upgrade 进程按状态机执行升级。
//
// 状态机：0 未升级 -> 1 待升级 -> 2 升级中 -> 3 升级完成 / 4 升级失败（可重试 4 -> 1）。
type Upgrade struct {
	cfg          *Config
	controller   upgradecontroller.Interface
	rtProvider   func() (runtime.Runtime, error)
	buildVersion string
}

// New 用已构造好的容器运行时创建 Upgrade（独立 upgrade 进程使用）。
func New(cfg *Config, rt runtime.Runtime, factory db.ShareDaoFactory) *Upgrade {
	return &Upgrade{
		cfg:        cfg,
		controller: upgradecontroller.New(factory.Upgrade()),
		rtProvider: func() (runtime.Runtime, error) { return rt, nil },
	}
}

// NewWithDefaultRuntime 使用进程内全局懒加载的容器运行时（pixiu 主服务内做版本检测使用），
// 避免启动阶段就建立 CRI 连接。buildVersion 为当前 pixiu 编译版本，用于版本检测请求参数（current_commit）。
func NewWithDefaultRuntime(cfg *Config, factory db.ShareDaoFactory, buildVersion string) *Upgrade {
	return &Upgrade{
		cfg:          cfg,
		controller:   upgradecontroller.New(factory.Upgrade()),
		rtProvider:   runtime.Default,
		buildVersion: buildVersion,
	}
}

// RunExecutor 启动升级请求轮询
func (u *Upgrade) RunExecutor(ctx context.Context) {
	klog.Infof("pixiu upgrade started: poll=%s container=%s upgrade_id=%s", u.cfg.PollInterval, u.cfg.ContainerName, u.cfg.UpgradeID)

	// 清理自身进程上次崩溃遗留的「升级中」状态，避免记录永远卡在 2
	if reset, err := u.controller.Reset(ctx, u.cfg.UpgradeID, "upgrade 进程重启，上次升级未完成"); err != nil {
		klog.Errorf("清理遗留升级状态失败: %v", err)
	} else if reset {
		klog.Warning("检测到上次升级未完成（状态 2），已回写为失败（状态 4），允许用户重试")
	}

	ticker := time.NewTicker(u.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.Info("pixiu-upgrade stopped")
			return
		case <-ticker.C:
			u.pollAndUpgrade(ctx)
		}
	}
}

// ProbeVersion 执行获取远端版本的记录。
func (u *Upgrade) ProbeVersion(ctx context.Context) error {
	// 读取 is_current 记录，作为本次拨测的当前版本参数
	currentRecord, err := u.controller.GetBy(ctx, db.WithUpgradeIsCurrent(true))
	if err != nil {
		klog.Errorf("查询当前使用版本失败: %v", err)
		return nil
	}
	currentVersion := ""
	if currentRecord != nil {
		currentVersion = currentRecord.VersionTag
	}
	currentCommit := ""
	if commit := strings.SplitN(u.buildVersion, "-", 2); len(commit) > 0 {
		currentCommit = commit[0]
	}

	latest, err := fetchLatestVersion(ctx, u.cfg.VersionURL, currentVersion, currentCommit)
	if err != nil {
		klog.Errorf("获取最新版本失败: %v", err)
		return nil
	}

	// 版本新旧以发布时间判定，避免版本号语义差异导致误判；
	// 任一侧缺少可解析的发布时间时保持原逻辑（继续查重/入库）。
	latestReleaseTime := parseReleaseTime(latest.ReleaseTime)
	if latestReleaseTime != nil && currentRecord != nil && currentRecord.ReleaseTime != nil &&
		!latestReleaseTime.After(*currentRecord.ReleaseTime) {
		klog.V(4).Infof("版本源返回的发布时间不晚于当前版本，跳过入库: latest=%s release_time=%s, current=%s release_time=%s",
			latest.Version, latestReleaseTime.Format(time.RFC3339),
			currentRecord.VersionTag, currentRecord.ReleaseTime.Format(time.RFC3339))
		return nil
	}

	// 尝试去获取本地是否已经这个版本
	record, err := u.controller.GetBy(ctx, db.WithUpgradeVersionTag(latest.Version), db.WithUpgradeCommitID(latest.CommitID))
	if err != nil {
		klog.Errorf("查询版本记录失败: %v", err)
		return nil
	}
	if record != nil {
		// 已经存在，则直接反馈不做处理
		return nil
	}

	image, err := u.resolveImage(latest)
	if err != nil {
		klog.Errorf("解析升级镜像失败: %v", err)
		return nil
	}
	newRecord := &model.UpgradeRecord{
		VersionTag:  latest.Version,
		CommitID:    latest.CommitID,
		Image:       image,
		Status:      model.UpgradeStatusNotUpgraded,
		ReleaseTime: parseReleaseTime(latest.ReleaseTime),
	}
	if err = u.controller.Create(ctx, newRecord); err != nil {
		klog.Errorf("保存版本记录失败: %v", err)
	}
	return nil
}

// Interval 返回版本检查周期，供 JobManager 注册调度使用。
func (u *Upgrade) Interval() time.Duration {
	return u.cfg.Interval
}

// pollAndUpgrade 轮询一次待升级请求，认领成功后异步执行升级。
func (u *Upgrade) pollAndUpgrade(ctx context.Context) {
	oneWaitForUpgrade, err := u.controller.Claim(ctx, u.cfg.UpgradeID)
	if err != nil {
		klog.Errorf("认领升级请求失败: %v", err)
		return
	}
	if oneWaitForUpgrade == nil {
		return
	}

	klog.Infof("收到升级请求，开始升级到 %s（image=%s upgrade_id=%s），数据库状态已置为「升级中」(status=2)", oneWaitForUpgrade.VersionTag, oneWaitForUpgrade.Image, oneWaitForUpgrade.UpgradeID)
	go u.doUpgrade(oneWaitForUpgrade.Image, oneWaitForUpgrade.VersionTag, oneWaitForUpgrade.UpgradeID)
}

func (u *Upgrade) doUpgrade(image, latestVersion, upgradeID string) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer cancel()

	klog.Infof("[升级] 开始执行: upgrade_id=%s version=%s image=%s container=%s timeout=%s", upgradeID, latestVersion, image, u.cfg.ContainerName, upgradeTimeout)

	// 状态回写使用独立短超时，避免升级主流程超时后无法记录失败原因
	markFailed := func(err error) {
		klog.Errorf("[升级] 升级失败（耗时 %s）: %v", time.Since(startedAt).Round(time.Millisecond), err)
		markCtx, markCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer markCancel()
		marked, markErr := u.controller.MarkFailed(markCtx, upgradeID, err.Error())
		if markErr != nil {
			klog.Errorf("[升级] 回写失败状态失败: %v", markErr)
		} else if !marked {
			klog.Warningf("[升级] 回写失败状态未命中: upgrade_id=%s", upgradeID)
		} else {
			klog.Infof("[升级] 已回写失败状态（status=4）: upgrade_id=%s", upgradeID)
		}
	}

	klog.Infof("[升级] 步骤 1/4：获取容器运行时")
	rt, err := u.rtProvider()
	if err != nil {
		markFailed(err)
		return
	}
	klog.Infof("[升级] 步骤 1/4：容器运行时就绪")

	klog.Infof("[升级] 步骤 2/4：拉取镜像并重建容器 container=%s image=%s", u.cfg.ContainerName, image)
	stepStartedAt := time.Now()
	oldImage, err := rt.ReplaceContainer(ctx, u.cfg.ContainerName, image)
	if err != nil {
		markFailed(err)
		return
	}
	klog.Infof("[升级] 步骤 2/4：容器重建完成（旧镜像=%s，耗时 %s）", oldImage, time.Since(stepStartedAt).Round(time.Millisecond))

	// 重建后短窗口内确认容器保持运行：退出/崩溃即判定升级失败
	klog.Infof("[升级] 步骤 3/4：观察新容器运行状态（窗口 %s，间隔 %s）", startupConfirmTimeout, startupConfirmInterval)
	stepStartedAt = time.Now()
	if err = u.waitForContainerStarted(ctx, rt); err != nil {
		markFailed(err)
		return
	}
	klog.Infof("[升级] 步骤 3/4：新容器运行稳定（耗时 %s）", time.Since(stepStartedAt).Round(time.Millisecond))

	// 容器稳定运行后由 upgrade 进程原子回写完成状态并切换 is_current。
	klog.Infof("[升级] 步骤 4/4：回写升级完成状态（status 2→3 并切换 is_current）")
	markCtx, markCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer markCancel()
	marked, markErr := u.controller.MarkCompleted(markCtx, upgradeID, latestVersion)
	if markErr != nil {
		klog.Errorf("[升级] 回写升级完成状态失败: %v", markErr)
	} else if !marked {
		klog.Warningf("[升级] 回写升级完成状态未命中: upgrade_id=%s version=%s", upgradeID, latestVersion)
	} else {
		klog.Infof("pixiu 升级完成: version=%s upgrade_id=%s（总耗时 %s）", latestVersion, upgradeID, time.Since(startedAt).Round(time.Millisecond))
	}
}

// waitForContainerStarted 在重建后观察窗口内确认容器保持运行；容器退出即视为启动失败。
func (u *Upgrade) waitForContainerStarted(ctx context.Context, rt runtime.Runtime) error {
	klog.Infof("[升级] 开始检查容器(%s)是否保持运行（窗口 %s，间隔 %s）", u.cfg.ContainerName, startupConfirmTimeout, startupConfirmInterval)
	deadline := time.Now().Add(startupConfirmTimeout)
	for {
		running, err := rt.IsContainerRunning(ctx, u.cfg.ContainerName)
		if err != nil {
			return fmt.Errorf("检查新版本容器状态失败: %v", err)
		}
		if !running {
			klog.Warningf("[升级] 容器(%s)未保持运行，判定升级失败", u.cfg.ContainerName)
			return fmt.Errorf("新版本容器未保持运行，升级失败")
		}
		if time.Now().After(deadline) {
			klog.Infof("[升级] 容器(%s)持续运行已超过 %s", u.cfg.ContainerName, startupConfirmTimeout)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(startupConfirmInterval):
		}
	}
}

func (u *Upgrade) resolveImage(info *VersionInfo) (string, error) {
	if info.Image != "" {
		return info.Image, nil
	}
	if u.cfg.ImageRepository == "" {
		return "", fmt.Errorf("版本源未返回镜像地址，且未配置 upgrade.image_repository")
	}
	return fmt.Sprintf("%s:%s", u.cfg.ImageRepository, info.Version), nil
}
