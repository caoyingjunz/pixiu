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

package updater

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/runtime"
)

const (
	upgradeTimeout = 30 * time.Minute
	// rollbackTimeout 回滚使用独立超时：升级主流程超时/取消后仍能完成回滚
	rollbackTimeout = 5 * time.Minute

	// completeMarkAttempts/Interval 新版 pixiu 启动后回写升级完成的重试策略
	completeMarkAttempts = 10
	completeMarkInterval = 3 * time.Second
)

// missingStatus 表示数据库中没有升级记录，供 decideCheckStatus 做纯函数判定。
const missingStatus = -1

// Updater 定时检查外部版本并持久化到数据库；用户确认后由独立 updater 进程按状态机执行升级。
//
// 状态机：0 未升级 -> 1 待升级 -> 2 升级中 -> 3 升级完成 / 4 升级失败（可重试 4 -> 1）。
type Updater struct {
	cfg          *Config
	factory      db.ShareDaoFactory
	rtProvider   func() (runtime.Runtime, error)
	buildVersion string
}

// New 用已构造好的容器运行时创建 Updater（独立 updater 进程使用）。
func New(cfg *Config, rt runtime.Runtime, factory db.ShareDaoFactory) *Updater {
	return &Updater{
		cfg:        cfg,
		factory:    factory,
		rtProvider: func() (runtime.Runtime, error) { return rt, nil },
	}
}

// NewWithDefaultRuntime 使用进程内全局懒加载的容器运行时（pixiu 主服务内做版本检测使用），
// 避免启动阶段就建立 CRI 连接。buildVersion 为当前 pixiu 编译版本，用于启动后回写升级完成状态。
func NewWithDefaultRuntime(cfg *Config, factory db.ShareDaoFactory, buildVersion string) *Updater {
	return &Updater{
		cfg:          cfg,
		factory:      factory,
		rtProvider:   runtime.Default,
		buildVersion: buildVersion,
	}
}

func (u *Updater) containerRuntime() (runtime.Runtime, error) {
	return u.rtProvider()
}

// RunChecker 启动版本检测循环，直到 ctx 被取消。
// 由 pixiu 主服务流程调用：定期查询外部版本号并与当前容器镜像比对，结果落库。
func (u *Updater) RunChecker(ctx context.Context) {
	klog.Infof("pixiu upgrade checker started: version_url=%s interval=%s container=%s",
		u.cfg.VersionURL, u.cfg.Interval, u.cfg.ContainerName)

	u.refresh(ctx)

	ticker := time.NewTicker(u.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.Info("pixiu upgrade checker stopped")
			return
		case <-ticker.C:
			u.refresh(ctx)
		}
	}
}

// RunExecutor 启动升级请求轮询，直到 ctx 被取消。
// 由独立的 pixiu-updater 进程调用：只负责按数据库状态执行升级，不做版本检测。
func (u *Updater) RunExecutor(ctx context.Context) {
	klog.Infof("pixiu updater started: poll=%s container=%s upgrader_id=%s",
		u.cfg.PollInterval, u.cfg.ContainerName, u.cfg.UpgraderID)

	// 清理自身进程上次崩溃遗留的「升级中」状态，避免记录永远卡在 2
	if reset, err := u.factory.Upgrade().ResetStale(ctx, u.cfg.UpgraderID, "upgrader 进程重启，上次升级未完成"); err != nil {
		klog.Errorf("清理遗留升级状态失败: %v", err)
	} else if reset {
		klog.Warning("检测到上次升级未完成（状态 2），已回写为失败（状态 4），允许用户重试")
	}

	ticker := time.NewTicker(u.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.Info("pixiu updater stopped")
			return
		case <-ticker.C:
			u.pollUpgradeRequest(ctx)
		}
	}
}

// refresh 查询最新版本并写入数据库，不执行升级。
func (u *Updater) refresh(ctx context.Context) {
	record, err := u.factory.Upgrade().Get(ctx)
	if err != nil {
		klog.Errorf("读取升级状态失败: %v", err)
		return
	}

	now := time.Now()
	// 升级流程进行中（待升级/升级中）：只刷新检查时间，绝不覆盖状态与目标版本。
	// SaveCheck 内部同样按状态 1/2 做条件保护。
	if record != nil &&
		(record.Status == model.UpgradeStatusPending || record.Status == model.UpgradeStatusUpgrading) {
		record.LastCheckedAt = &now
		u.saveCheck(ctx, record)
		return
	}

	latest, err := fetchLatestVersion(ctx, u.cfg.VersionURL)
	if err != nil {
		u.saveCheckError(ctx, err.Error())
		klog.Errorf("检查最新版本失败: %v", err)
		return
	}
	image, err := u.resolveImage(latest)
	if err != nil {
		u.saveCheckError(ctx, err.Error())
		klog.Errorf("解析升级镜像失败: %v", err)
		return
	}
	rt, err := u.containerRuntime()
	if err != nil {
		message := fmt.Sprintf("初始化容器运行时失败: %v", err)
		u.saveCheckError(ctx, message)
		klog.Errorf("%s", message)
		return
	}
	current, err := rt.ContainerImage(ctx, u.cfg.ContainerName)
	if err != nil {
		message := fmt.Sprintf("获取当前容器(%s)镜像失败: %v", u.cfg.ContainerName, err)
		u.saveCheckError(ctx, message)
		klog.Errorf("%s", message)
		return
	}

	currentVersion := imageTag(current)
	available := compareVersion(latest.Version, currentVersion) > 0
	currentStatus := missingStatus
	if record != nil {
		currentStatus = record.Status
	}
	sameVersion := record != nil && compareVersion(record.LatestVersion, latest.Version) == 0
	status := decideCheckStatus(currentStatus, available, sameVersion)

	newRecord := &model.UpgradeRecord{
		CurrentVersion:   currentVersion,
		LatestVersion:    latest.Version,
		Image:            image,
		UpgradeAvailable: available,
		Status:           status,
		LastError:        checkStatusLastError(status, record),
		LastCheckedAt:    &now,
	}
	u.saveCheck(ctx, newRecord)

	if available {
		klog.Infof("发现新版本: current=%s latest=%s image=%s status=%d", currentVersion, latest.Version, image, status)
	} else {
		klog.Infof("pixiu 已是最新版本: current=%s latest=%s status=%d", currentVersion, latest.Version, status)
	}
}

// decideCheckStatus 依据当前状态与版本检查结果计算检查器应写入的状态。
// currentStatus 为 missingStatus 表示记录不存在；available 表示发现更新版本；
// sameVersion 表示本次最新版本与记录中的目标版本一致（用于识别同一版本的重复失败）。
func decideCheckStatus(currentStatus int, available, sameVersion bool) int {
	if available {
		// 同一版本曾升级失败：保持失败态等待用户重试，避免检查器把失败重置为 0
		if currentStatus == model.UpgradeStatusFailed && sameVersion {
			return model.UpgradeStatusFailed
		}
		return model.UpgradeStatusNotUpgraded
	}
	switch currentStatus {
	case model.UpgradeStatusNotUpgraded, model.UpgradeStatusFailed, missingStatus:
		// 无可用升级：当前已是目标版本，标记为升级完成（含首次创建记录）
		return model.UpgradeStatusCompleted
	default:
		// 保持原状态（已完成为 3；1/2 正常会在网络请求前提前返回，不会到达这里）
		return currentStatus
	}
}

// checkStatusLastError 返回检查器写入记录时应保留的错误信息：
// 同一版本升级失败（状态 4）时保留上一轮失败原因供页面展示，其他状态清空。
func checkStatusLastError(status int, record *model.UpgradeRecord) string {
	if status == model.UpgradeStatusFailed && record != nil {
		return record.LastError
	}
	return ""
}

// matchUpgradeTarget 判断当前运行镜像/编译版本是否已到达记录中的升级目标。
func matchUpgradeTarget(currentImage, buildVersion string, record *model.UpgradeRecord) bool {
	if record == nil {
		return false
	}
	if compareVersion(imageTag(currentImage), record.LatestVersion) == 0 {
		return true
	}
	if currentImage != "" && currentImage == record.Image {
		return true
	}
	if buildVersion != "" && compareVersion(buildVersion, record.LatestVersion) == 0 {
		return true
	}
	return false
}

// pollUpgradeRequest 原子认领待升级请求（状态 1 -> 2）并执行升级。
func (u *Updater) pollUpgradeRequest(ctx context.Context) {
	record, claimed, err := u.factory.Upgrade().Claim(ctx, u.cfg.UpgraderID)
	if err != nil {
		klog.Errorf("认领升级请求失败: %v", err)
		return
	}
	if !claimed {
		return
	}

	klog.Infof("收到升级请求，开始升级到 %s（image=%s upgrader=%s）",
		record.LatestVersion, record.Image, record.UpgraderID)
	go u.doUpgrade(record.Image, record.LatestVersion, record.UpgraderID)
}

func (u *Updater) doUpgrade(image, latestVersion, upgraderID string) {
	ctx, cancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer cancel()

	// 状态回写使用独立短超时，避免升级主流程超时后无法记录失败原因
	markFailed := func(err error) {
		klog.Errorf("升级失败: %v", err)
		markCtx, markCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer markCancel()
		if _, markErr := u.factory.Upgrade().MarkFailed(markCtx, upgraderID, err.Error()); markErr != nil {
			klog.Errorf("回写升级失败状态失败: %v", markErr)
		}
	}

	rt, err := u.containerRuntime()
	if err != nil {
		markFailed(err)
		return
	}

	oldImage, err := rt.ReplaceContainer(ctx, u.cfg.ContainerName, image)
	if err == nil {
		err = u.healthCheck(ctx)
	}
	if err != nil {
		// 回滚使用独立 ctx：主流程超时/取消后仍需完成回滚
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), rollbackTimeout)
		defer rollbackCancel()

		// 拉取失败等场景 ReplaceContainer 在删除旧容器前就失败，旧容器仍在运行：
		// 此时跳过回滚重建，避免一次不必要的服务中断（仅精确相等才跳过，防止镜像引用归一化差异误判）
		current, curErr := rt.ContainerImage(rollbackCtx, u.cfg.ContainerName)
		if curErr == nil && oldImage != "" && current == oldImage {
			klog.Warningf("升级失败但容器仍运行旧镜像 %s，跳过回滚重建", oldImage)
		} else {
			u.rollback(rollbackCtx, oldImage)
		}
		markFailed(err)
		return
	}

	// 升级成功不在此写状态 3：由新版本 pixiu 启动后确认运行版本与目标版本一致再回写
	klog.Infof("pixiu 已重建为 %s（目标版本 %s），等待新 pixiu 启动回写状态 3", image, latestVersion)
}

// MarkCompleted 由新版 pixiu 启动后调用：确认当前运行版本已达到升级目标后回写状态 3。
// 最多重试 completeMarkAttempts 次，调用方应传入约 60 秒超时的 ctx。
func (u *Updater) MarkCompleted(ctx context.Context) {
	record, err := u.factory.Upgrade().Get(ctx)
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
					current, record.LatestVersion, lastErr)
				return
			case <-time.After(completeMarkInterval):
			}
		}

		rt, rtErr := u.containerRuntime()
		if rtErr != nil {
			lastErr = rtErr
			continue
		}
		current, rtErr = rt.ContainerImage(ctx, u.cfg.ContainerName)
		if rtErr != nil {
			lastErr = rtErr
			continue
		}
		if !matchUpgradeTarget(current, u.buildVersion, record) {
			lastErr = fmt.Errorf("当前运行镜像(%s)尚未匹配目标版本(%s)", current, record.LatestVersion)
			continue
		}

		marked, markErr := u.factory.Upgrade().MarkCompleted(ctx, record.UpgraderID, record.LatestVersion)
		if markErr != nil {
			lastErr = markErr
			continue
		}
		if !marked {
			// 状态已被其他流程更新（如已完成或已重置），无需继续重试
			klog.Warningf("回写升级完成状态未命中: upgrader=%s target=%s", record.UpgraderID, record.LatestVersion)
			return
		}
		klog.Infof("pixiu 升级完成: version=%s upgrader=%s", record.LatestVersion, record.UpgraderID)
		return
	}

	klog.Warningf("回写升级完成状态失败: current=%s target=%s last_err=%v",
		current, record.LatestVersion, lastErr)
}

func (u *Updater) rollback(ctx context.Context, oldImage string) {
	if oldImage == "" {
		klog.Warning("跳过回滚：旧镜像未知")
		return
	}
	rt, err := u.containerRuntime()
	if err != nil {
		klog.Errorf("回滚失败: %v", err)
		return
	}
	klog.Warningf("开始回滚到旧镜像 %s", oldImage)
	if _, err = rt.ReplaceContainer(ctx, u.cfg.ContainerName, oldImage); err != nil {
		klog.Errorf("回滚失败: %v", err)
		return
	}
	klog.Infof("已回滚到旧镜像 %s", oldImage)
}

func (u *Updater) saveCheck(ctx context.Context, record *model.UpgradeRecord) {
	if err := u.factory.Upgrade().SaveCheck(ctx, record); err != nil {
		klog.Errorf("保存升级状态失败: %v", err)
	}
}

func (u *Updater) saveCheckError(ctx context.Context, message string) {
	if err := u.factory.Upgrade().SaveCheckError(ctx, message); err != nil {
		klog.Errorf("保存版本检查错误失败: %v", err)
	}
}

func (u *Updater) resolveImage(info *VersionInfo) (string, error) {
	if info.Image != "" {
		return info.Image, nil
	}
	if u.cfg.ImageRepository == "" {
		return "", fmt.Errorf("版本源未返回镜像地址，且未配置 upgrade.image_repository")
	}
	return fmt.Sprintf("%s:%s", u.cfg.ImageRepository, info.Version), nil
}

func (u *Updater) healthCheck(ctx context.Context) error {
	if u.cfg.HealthCheckURL == "" {
		return nil
	}

	var lastErr error
	for i := 0; i < u.cfg.HealthCheckRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		checkCtx, cancel := context.WithTimeout(ctx, u.cfg.HealthCheckTimeout)
		req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, u.cfg.HealthCheckURL, nil)
		if err != nil {
			cancel()
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				cancel()
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		cancel()
		klog.Warningf("健康检查未通过(第 %d 次): %v", i+1, lastErr)
	}
	return lastErr
}
