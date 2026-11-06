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

const upgradeTimeout = 30 * time.Minute

// Updater 定时检查外部版本并持久化到数据库；用户确认后（数据库标记 requested）再执行升级。
type Updater struct {
	cfg     *Config
	rt      runtime.Runtime
	factory db.ShareDaoFactory
}

func New(cfg *Config, rt runtime.Runtime, factory db.ShareDaoFactory) *Updater {
	return &Updater{cfg: cfg, rt: rt, factory: factory}
}

// Run 启动版本检查与升级请求轮询，直到 ctx 被取消。
func (u *Updater) Run(ctx context.Context) {
	klog.Infof("pixiu-updater started: version_url=%s interval=%s poll=%s container=%s",
		u.cfg.VersionURL, u.cfg.Interval, u.cfg.PollInterval, u.cfg.ContainerName)

	u.refresh(ctx)

	versionTicker := time.NewTicker(u.cfg.Interval)
	requestTicker := time.NewTicker(u.cfg.PollInterval)
	defer versionTicker.Stop()
	defer requestTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("pixiu-updater stopped")
			return
		case <-versionTicker.C:
			u.refresh(ctx)
		case <-requestTicker.C:
			u.pollUpgradeRequest(ctx)
		}
	}
}

// refresh 查询最新版本并写入数据库，不执行升级。
func (u *Updater) refresh(ctx context.Context) {
	record, err := u.loadRecord(ctx)
	if err != nil {
		klog.Errorf("读取升级状态失败: %v", err)
		return
	}
	now := time.Now()
	record.LastCheckedAt = &now

	latest, err := fetchLatestVersion(ctx, u.cfg.VersionURL)
	if err != nil {
		record.LastError = err.Error()
		u.saveRecord(ctx, record)
		klog.Errorf("检查最新版本失败: %v", err)
		return
	}
	image, err := u.resolveImage(latest)
	if err != nil {
		record.LastError = err.Error()
		u.saveRecord(ctx, record)
		klog.Errorf("解析升级镜像失败: %v", err)
		return
	}
	current, err := u.rt.ContainerImage(ctx, u.cfg.ContainerName)
	if err != nil {
		record.LastError = fmt.Sprintf("获取当前容器(%s)镜像失败: %v", u.cfg.ContainerName, err)
		u.saveRecord(ctx, record)
		klog.Errorf("%s", record.LastError)
		return
	}

	currentVersion := imageTag(current)
	record.CurrentVersion = currentVersion
	record.LatestVersion = latest.Version
	record.Image = image
	record.UpgradeAvailable = compareVersion(latest.Version, currentVersion) > 0
	record.LastError = ""
	u.saveRecord(ctx, record)

	if record.UpgradeAvailable {
		klog.Infof("发现新版本: current=%s latest=%s image=%s", currentVersion, latest.Version, image)
	} else {
		klog.Infof("pixiu 已是最新版本: current=%s latest=%s", currentVersion, latest.Version)
	}
}

// pollUpgradeRequest 检查数据库中的升级请求，用户确认后执行升级。
func (u *Updater) pollUpgradeRequest(ctx context.Context) {
	record, err := u.factory.Upgrade().Get(ctx)
	if err != nil {
		klog.Errorf("读取升级状态失败: %v", err)
		return
	}
	if record == nil || !record.Requested || record.Upgrading {
		return
	}
	if !record.UpgradeAvailable || record.Image == "" {
		return
	}

	record.Requested = false
	record.Upgrading = true
	record.LastError = ""
	if err = u.factory.Upgrade().Save(ctx, record); err != nil {
		klog.Errorf("更新升级状态失败: %v", err)
		return
	}
	klog.Infof("收到升级请求，开始升级到 %s", record.LatestVersion)
	go u.doUpgrade(record.Image, record.LatestVersion)
}

func (u *Updater) doUpgrade(image, latestVersion string) {
	ctx, cancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer cancel()

	oldImage, err := u.rt.ReplaceContainer(ctx, u.cfg.ContainerName, image)
	if err == nil {
		err = u.healthCheck(ctx)
	}
	if err != nil {
		klog.Errorf("升级失败: %v", err)
		u.rollback(ctx, oldImage)
		u.updateRecord(ctx, func(record *model.UpgradeRecord) {
			now := time.Now()
			record.Upgrading = false
			record.LastError = err.Error()
			record.LastCheckedAt = &now
		})
		return
	}

	u.updateRecord(ctx, func(record *model.UpgradeRecord) {
		now := time.Now()
		record.Upgrading = false
		record.Requested = false
		record.CurrentVersion = latestVersion
		record.UpgradeAvailable = false
		record.LastError = ""
		record.LastCheckedAt = &now
	})
	klog.Infof("pixiu 已升级到 %s", latestVersion)
}

func (u *Updater) rollback(ctx context.Context, oldImage string) {
	if oldImage == "" {
		klog.Warning("跳过回滚：旧镜像未知")
		return
	}
	klog.Warningf("开始回滚到旧镜像 %s", oldImage)
	if _, err := u.rt.ReplaceContainer(ctx, u.cfg.ContainerName, oldImage); err != nil {
		klog.Errorf("回滚失败: %v", err)
		return
	}
	klog.Infof("已回滚到旧镜像 %s", oldImage)
}

func (u *Updater) loadRecord(ctx context.Context) (*model.UpgradeRecord, error) {
	record, err := u.factory.Upgrade().Get(ctx)
	if err != nil {
		return nil, err
	}
	if record == nil {
		record = &model.UpgradeRecord{}
	}
	return record, nil
}

func (u *Updater) updateRecord(ctx context.Context, update func(*model.UpgradeRecord)) {
	record, err := u.loadRecord(ctx)
	if err != nil {
		klog.Errorf("读取升级状态失败: %v", err)
		return
	}
	update(record)
	u.saveRecord(ctx, record)
}

func (u *Updater) saveRecord(ctx context.Context, record *model.UpgradeRecord) {
	if err := u.factory.Upgrade().Save(ctx, record); err != nil {
		klog.Errorf("保存升级状态失败: %v", err)
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
