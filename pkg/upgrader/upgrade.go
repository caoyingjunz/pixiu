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

	// completeMarkAttempts/Interval 新版 pixiu 启动后回写升级完成的重试策略
	completeMarkAttempts = 10
	completeMarkInterval = 3 * time.Second
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
// 避免启动阶段就建立 CRI 连接。buildVersion 为当前 pixiu 编译版本，用于启动后回写升级完成状态。
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

// matchUpgradeTarget 判断当前运行镜像/编译版本是否已到达记录中的升级目标。
func matchUpgradeTarget(currentImage, buildVersion string, record *model.UpgradeRecord) bool {
	if record == nil {
		return false
	}
	if compareVersion(imageTag(currentImage), record.VersionTag) == 0 {
		return true
	}
	if currentImage != "" && currentImage == record.Image {
		return true
	}
	if buildVersion != "" && compareVersion(buildVersion, record.VersionTag) == 0 {
		return true
	}
	return false
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

	klog.Infof("收到升级请求，开始升级到 %s（image=%s upgrade_id=%s）", oneWaitForUpgrade.VersionTag, oneWaitForUpgrade.Image, oneWaitForUpgrade.UpgradeID)
	go u.doUpgrade(oneWaitForUpgrade.Image, oneWaitForUpgrade.VersionTag, oneWaitForUpgrade.UpgradeID)
}

func (u *Upgrade) doUpgrade(image, latestVersion, upgradeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer cancel()

	// 状态回写使用独立短超时，避免升级主流程超时后无法记录失败原因
	markFailed := func(err error) {
		klog.Errorf("升级失败: %v", err)
		markCtx, markCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer markCancel()
		if _, markErr := u.controller.MarkFailed(markCtx, upgradeID, err.Error()); markErr != nil {
			klog.Errorf("回写升级失败状态失败: %v", markErr)
		}
	}

	rt, err := u.rtProvider()
	if err != nil {
		markFailed(err)
		return
	}

	if _, err = rt.ReplaceContainer(ctx, u.cfg.ContainerName, image); err != nil {
		markFailed(err)
		return
	}

	// 重建后短窗口内确认容器保持运行：退出/崩溃即判定升级失败
	if err = u.waitForContainerStarted(ctx, rt); err != nil {
		markFailed(err)
		return
	}
	// 升级成功不在此写状态 3：由新版本 pixiu 启动后确认运行版本与目标版本一致再回写
	klog.Infof("pixiu 已重建为 %s（目标版本 %s），等待新 pixiu 启动回写状态 3", image, latestVersion)
}

// waitForContainerStarted 在重建后观察窗口内确认容器保持运行；容器退出即视为启动失败。
func (u *Upgrade) waitForContainerStarted(ctx context.Context, rt runtime.Runtime) error {
	deadline := time.Now().Add(startupConfirmTimeout)
	for {
		running, err := rt.IsContainerRunning(ctx, u.cfg.ContainerName)
		if err != nil {
			return fmt.Errorf("检查新版本容器状态失败: %v", err)
		}
		if !running {
			return fmt.Errorf("新版本容器未保持运行，升级失败")
		}
		if time.Now().After(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(startupConfirmInterval):
		}
	}
}

// MarkCompleted 由新版 pixiu 启动后调用：确认当前运行版本已达到升级目标后回写状态 3。
// 最多重试 completeMarkAttempts 次，调用方应传入约 60 秒超时的 ctx。
func (u *Upgrade) MarkCompleted(ctx context.Context) {
	record, err := u.controller.GetBy(ctx, db.WithUpgradeStatus(model.UpgradeStatusUpgrading))
	if err != nil {
		klog.Errorf("读取升级状态失败: %v", err)
		return
	}
	if record == nil || record.Status != model.UpgradeStatusUpgrading {
		return
	}

	var lastErr error
	var current string
	for i := 0; i < completeMarkAttempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				klog.Warningf("回写升级完成状态取消: current=%s target=%s last_err=%v",
					current, record.VersionTag, lastErr)
				return
			case <-time.After(completeMarkInterval):
			}
		}

		rt, rtErr := u.rtProvider()
		if rtErr != nil {
			lastErr = rtErr
			continue
		}
		current, rtErr = rt.ContainerImageRef(ctx, u.cfg.ContainerName)
		if rtErr != nil {
			lastErr = rtErr
			continue
		}
		if !matchUpgradeTarget(current, u.buildVersion, record) {
			lastErr = fmt.Errorf("当前运行镜像(%s)尚未匹配目标版本(%s)", current, record.VersionTag)
			continue
		}

		marked, markErr := u.controller.MarkCompleted(ctx, record.UpgradeID, record.VersionTag)
		if markErr != nil {
			lastErr = markErr
			continue
		}
		if !marked {
			// 状态已被其他流程更新（如已完成或已重置），无需继续重试
			klog.Warningf("回写升级完成状态未命中: upgrade_id=%s target=%s", record.UpgradeID, record.VersionTag)
			return
		}
		klog.Infof("pixiu 升级完成: version=%s upgrade_id=%s", record.VersionTag, record.UpgradeID)
		return
	}

	klog.Warningf("回写升级完成状态失败: current=%s target=%s last_err=%v",
		current, record.VersionTag, lastErr)
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
