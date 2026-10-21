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

package apps

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	helmchart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"sigs.k8s.io/yaml"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/pkg/chartutil"
	controllerutil "github.com/caoyingjunz/pixiu/pkg/controller/util"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

const (
	defaultPageSize    = 10
	maxPageSize        = 100
	defaultMaxUploadMB = 32
)

// precheckChart 上传前置检查：存储就绪 → 管理员权限 → 操作用户。
func (c *controller) precheckChart(ctx context.Context) (*model.User, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return nil, err
	}

	user, err := httputils.GetUserFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// parseChart 上传包解析校验：解析元数据 → 仅接受 application 类型 → name/version 必填。
func parseChart(data []byte) (*chartutil.Metadata, error) {
	md, err := chartutil.Parse(data)
	if err != nil {
		return nil, apierrors.NewError(err, http.StatusBadRequest)
	}
	if !chartutil.IsApplication(md) {
		return nil, apierrors.NewError(fmt.Errorf("chart type %q is not installable, only application charts are accepted", md.Type), http.StatusBadRequest)
	}
	if md.Name == "" || md.Version == "" {
		return nil, apierrors.NewError(fmt.Errorf("chart name and version are required"), http.StatusBadRequest)
	}
	return md, nil
}

// Upload 上传本地 .tgz chart 包（root/admin）。
func (c *controller) Upload(ctx context.Context, _ string, category string, r io.Reader) (*types.ChartInfo, error) {
	user, err := c.precheckChart(ctx)
	if err != nil {
		return nil, err
	}

	data, err := c.readLimited(r)
	if err != nil {
		return nil, err
	}

	md, err := parseChart(data)
	if err != nil {
		return nil, err
	}
	return c.persistChart(ctx, md, data, category, "upload", "", user)
}

// persistChart 上传落库管线：校验重复 → 提取默认 values → 存储 → 写 charts/chart_versions。
func (c *controller) persistChart(ctx context.Context, md *chartutil.Metadata, data []byte, category, source, repoURL string, user *model.User) (*types.ChartInfo, error) {
	// 先提取内置 values 原文（任何写操作之前），失败直接 500，避免产生副作用。
	rawValues, err := archiveValuesFromData(data)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	chart, err := c.factory.Chart().GetChartByName(ctx, md.Name)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	// 新建目录项
	if chart == nil {
		chart = &model.Chart{
			Name:          md.Name,
			Category:      category,
			Description:   md.Description,
			Icon:          md.Icon,
			Source:        source,
			RepoURL:       repoURL,
			LatestVersion: md.Version,
			CreatedBy:     user.Id,
			CreatedByName: user.Name,
		}
		if chart, err = c.factory.Chart().CreateChart(ctx, chart); err != nil {
			return nil, apierrors.ErrServerInternal
		}
	} else {
		versions, err := c.factory.Chart().ListChartVersions(ctx, chart.Id)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		for _, v := range versions {
			if v.Version == md.Version {
				return nil, apierrors.NewError(fmt.Errorf("chart %s version %s already exists", md.Name, md.Version), http.StatusConflict)
			}
			// sha256 去重：相同内容包（跨版本）拒绝重复入库
			if v.Digest != "" && v.Digest == md.Sha256 {
				return nil, apierrors.NewError(fmt.Errorf("identical chart package already exists as version %s", v.Version), http.StatusConflict)
			}
		}
	}

	key := storeKey(md.Name, md.Version)
	if err = c.store.Save(ctx, key, bytes.NewReader(data)); err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to store chart: %w", err), http.StatusInternalServerError)
	}

	version := &model.AppstoreChartVersion{
		ChartId:     chart.Id,
		Version:     md.Version,
		AppVersion:  md.AppVersion,
		Description: md.Description,
		Icon:        md.Icon,
		Digest:      md.Sha256,
		Size:        md.Size,
		StorageKey:  key,
		// 内置 values.yaml 原文作为平台可编辑的默认配置基线，随版本行一次写入（天然原子）。
		DefaultValues: &rawValues,
	}
	if _, err = c.factory.Chart().CreateChartVersion(ctx, version); err != nil {
		// 回滚已存储的对象，避免产生孤儿文件
		_ = c.store.Delete(ctx, key)
		return nil, apierrors.ErrServerInternal
	}

	// 更新目录项的展示信息与最新版本
	updates := map[string]interface{}{"latest_version": md.Version}
	if category != "" {
		updates["category"] = category
	}
	if md.Description != "" {
		updates["description"] = md.Description
	}
	if md.Icon != "" {
		updates["icon"] = md.Icon
	}
	if source != "" {
		updates["source"] = source
	}
	if repoURL != "" {
		updates["repo_url"] = repoURL
	}
	if err = c.factory.Chart().UpdateChart(ctx, chart.Id, chart.ResourceVersion, updates); err != nil {
		return nil, apierrors.ErrServerInternal
	}

	return c.Get(ctx, chart.Id)
}

// List 分页查询 chart 目录。
func (c *controller) List(ctx context.Context, opt types.ListChartOption) (*types.ChartListResponse, error) {
	page, pageSize := normalizePaging(opt.Page, opt.PageSize)

	opts := []db.Options{
		db.WithNameLike(opt.Name),
		db.WithChartCategory(opt.Category),
	}
	total, err := c.factory.Chart().CountCharts(ctx, opts...)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	opts = append(opts, db.WithOrderByDesc(), db.WithOffset((page-1)*pageSize), db.WithLimit(pageSize))
	charts, err := c.factory.Chart().ListCharts(ctx, opts...)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}

	items := make([]types.ChartInfo, 0, len(charts))
	for i := range charts {
		items = append(items, toChartInfo(&charts[i], nil))
	}

	// 批量判定 installed：一次查询本页所有 chart 的安装记录，避免逐条 N+1
	if len(items) > 0 {
		ids := make([]int64, 0, len(items))
		for i := range items {
			ids = append(ids, items[i].Id)
		}
		installedIds, err := c.factory.Chart().ListInstalledChartIds(ctx, ids)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		installedSet := make(map[int64]struct{}, len(installedIds))
		for _, id := range installedIds {
			installedSet[id] = struct{}{}
		}
		for i := range items {
			if _, ok := installedSet[items[i].Id]; ok {
				items[i].Installed = true
			}
		}
	}

	return &types.ChartListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	}, nil
}

// Get 查看 chart 详情（含全部版本）。
func (c *controller) Get(ctx context.Context, id int64) (*types.ChartInfo, error) {
	chart, versions, err := c.chartFor(ctx, id)
	if err != nil {
		if utilerrors.IsRecordNotFound(err) {
			return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
		}
		return nil, apierrors.ErrServerInternal
	}
	if chart == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}
	info := toChartInfo(chart, versions)
	return &info, nil
}

// countInFlightInstalls 统计在途安装任务数（deploy_status 为 pending/installing）。
func countInFlightInstalls(installs []model.AppstoreInstall) int {
	var count int
	for i := range installs {
		if installs[i].DeployStatus == model.InstallDeployPending || installs[i].DeployStatus == model.InstallDeployInstalling {
			count++
		}
	}
	return count
}

// Delete 删除 chart 目录项（root/admin）。
// 级联策略：仅在存在在途安装任务（deploy_status 为 pending/installing）时拒绝删除（409）；
// 其余安装记账（含 success/failed）不阻止删除，记账记录保留，已安装应用与卸载链路不受影响。
// 删除时清理版本记录 + 目录项 + 存储对象。
func (c *controller) Delete(ctx context.Context, id int64) error {
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return err
	}

	chart, versions, err := c.chartFor(ctx, id)
	if err != nil {
		return apierrors.ErrServerInternal
	}
	if chart == nil {
		return apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}

	installs, err := c.factory.Chart().ListInstalls(ctx, db.WithChartId(chart.Id))
	if err != nil {
		return apierrors.ErrServerInternal
	}
	if busy := countInFlightInstalls(installs); busy > 0 {
		return apierrors.NewError(
			fmt.Errorf("chart %s has %d in-flight install operation(s), please retry after they finish", chart.Name, busy),
			http.StatusConflict,
		)
	}

	if err = c.factory.Chart().DeleteChartVersionsByChart(ctx, chart.Id); err != nil {
		return apierrors.ErrServerInternal
	}
	if err = c.factory.Chart().DeleteChart(ctx, chart.Id); err != nil {
		return apierrors.ErrServerInternal
	}

	// 存储对象删除失败只记录（DB 已清理，避免残留目录项）
	if c.store != nil {
		for _, v := range versions {
			_ = c.store.Delete(ctx, v.StorageKey)
		}
	}
	return nil
}

// DefaultValues 返回指定版本的默认配置。
// source=chart 时解析所选版本的 chart 包并返回包内 values.yaml 原文；
// 其余（含空值/未知值）按 source=db 处理，只返回版本行已落库的值，
// 存量版本为 NULL 时直接返回空值，不再解析 chart 包懒回填。
func (c *controller) DefaultValues(ctx context.Context, id int64, version string, source string) (*types.ChartValuesInfo, error) {
	cv, err := c.resolveVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(source) == "chart" {
		raw, err := c.chartPackageValues(ctx, cv)
		if err != nil {
			return nil, err
		}
		return toChartValuesInfo(raw, cv.ResourceVersion), nil
	}
	if cv.DefaultValues == nil {
		return toChartValuesInfo("", cv.ResourceVersion), nil
	}
	return toChartValuesInfo(*cv.DefaultValues, cv.ResourceVersion), nil
}

// UpdateDefaultValues 修改指定版本的平台默认配置（root/admin）。
func (c *controller) UpdateDefaultValues(ctx context.Context, id int64, version string, req *types.UpdateChartValuesRequest) (*types.ChartValuesInfo, error) {
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return nil, err
	}

	values := req.Values
	if strings.TrimSpace(values) == "" {
		values = "" // 空串/纯空白统一按清空处理
	} else {
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(values), &obj); err != nil {
			return nil, apierrors.NewError(fmt.Errorf("invalid values yaml: %w", err), http.StatusBadRequest)
		}
		if obj == nil {
			return nil, apierrors.NewError(fmt.Errorf("values yaml must be a mapping"), http.StatusBadRequest)
		}
	}

	cv, err := c.resolveVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}

	ok, err := c.factory.Chart().UpdateChartVersionValues(ctx, cv.Id, values, req.ResourceVersion)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if !ok {
		return nil, apierrors.NewError(fmt.Errorf("default values have been modified, please refresh and retry"), http.StatusConflict)
	}
	cv.DefaultValues = &values
	cv.ResourceVersion = req.ResourceVersion + 1
	return toChartValuesInfo(values, cv.ResourceVersion), nil
}

// ResetDefaultValues 将平台默认配置恢复为 chart 包内置的 values.yaml（root/admin）。
func (c *controller) ResetDefaultValues(ctx context.Context, id int64, version string) (*types.ChartValuesInfo, error) {
	if err := controllerutil.CheckAdmin(ctx); err != nil {
		return nil, err
	}
	cv, err := c.resolveVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	raw, err := c.chartPackageValues(ctx, cv)
	if err != nil {
		return nil, err
	}

	// 重置以当前版本行版本号为乐观锁；并发修改时重读并重试一次。
	expected := cv.ResourceVersion
	for attempt := 0; attempt < 2; attempt++ {
		ok, err := c.factory.Chart().UpdateChartVersionValues(ctx, cv.Id, raw, expected)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		if ok {
			cv.DefaultValues = &raw
			cv.ResourceVersion = expected + 1
			return toChartValuesInfo(raw, cv.ResourceVersion), nil
		}
		latest, err := c.factory.Chart().GetChartVersion(ctx, cv.ChartId, cv.Version)
		if err != nil {
			return nil, apierrors.ErrServerInternal
		}
		if latest == nil {
			return nil, apierrors.NewError(fmt.Errorf("chart version not found"), http.StatusNotFound)
		}
		expected = latest.ResourceVersion
	}
	return nil, apierrors.NewError(fmt.Errorf("default values have been modified, please refresh and retry"), http.StatusConflict)
}

func toChartValuesInfo(values string, resourceVersion int64) *types.ChartValuesInfo {
	return &types.ChartValuesInfo{Values: values, ResourceVersion: resourceVersion}
}

// chartPackageValues 读取 chart 包并返回内置 values.yaml 原文（缺失时回退解析后 values 的 YAML）。
func (c *controller) chartPackageValues(ctx context.Context, cv *model.AppstoreChartVersion) (string, error) {
	if err := c.ensureStore(); err != nil {
		return "", apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	rc, err := c.store.Open(ctx, cv.StorageKey)
	if err != nil {
		return "", apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", apierrors.ErrServerInternal
	}
	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return "", apierrors.NewError(fmt.Errorf("invalid chart archive: %w", err), http.StatusBadRequest)
	}
	return archiveValues(ch), nil
}

// archiveValuesFromData 解析 tgz 并提取内置 values.yaml 原文（导入落库路径使用）。
func archiveValuesFromData(data []byte) (string, error) {
	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return archiveValues(ch), nil
}

// archiveValues 取 chart 包内 values.yaml 原文；包内缺失时回退为解析后 values 的 YAML 序列化。
func archiveValues(ch *helmchart.Chart) string {
	if ch == nil {
		return ""
	}
	for _, f := range ch.Raw {
		if f != nil && f.Name == "values.yaml" && strings.TrimSpace(string(f.Data)) != "" {
			return string(f.Data)
		}
	}
	if len(ch.Values) == 0 {
		return ""
	}
	out, err := yaml.Marshal(ch.Values)
	if err != nil {
		return ""
	}
	return string(out)
}

// Templates 返回 chart 指定版本（version 为空取最新版本）内的模板文件清单（原始、未渲染）。
func (c *controller) Templates(ctx context.Context, id int64, version string) ([]types.ChartTemplateInfo, error) {
	ch, err := c.loadChart(ctx, id, version)
	if err != nil {
		return nil, err
	}
	out := make([]types.ChartTemplateInfo, 0, len(ch.Templates))
	for _, f := range ch.Templates {
		out = append(out, types.ChartTemplateInfo{
			Name: f.Name,
			Kind: templateKind(f.Data),
			Size: int64(len(f.Data)),
		})
	}
	return out, nil
}

// TemplateContent 返回 chart 指定版本（version 为空取最新版本）内指定模板文件的原始内容。
func (c *controller) TemplateContent(ctx context.Context, id int64, name, version string) (string, error) {
	ch, err := c.loadChart(ctx, id, version)
	if err != nil {
		return "", err
	}
	for _, f := range ch.Templates {
		if f.Name == name {
			return string(f.Data), nil
		}
	}
	return "", apierrors.NewError(fmt.Errorf("template %s not found", name), http.StatusNotFound)
}

// loadChart 取 chart 指定版本（version 为空取最新版本）的存储包并解析为 helm chart 对象（默认 values / 模板读取共用）。
func (c *controller) loadChart(ctx context.Context, id int64, version string) (*helmchart.Chart, error) {
	if err := c.ensureStore(); err != nil {
		return nil, apierrors.NewError(err, http.StatusServiceUnavailable)
	}
	cv, err := c.resolveVersion(ctx, id, version)
	if err != nil {
		return nil, err
	}
	rc, err := c.store.Open(ctx, cv.StorageKey)
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("chart package not found: %w", err), http.StatusNotFound)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	ch, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("invalid chart archive: %w", err), http.StatusBadRequest)
	}
	return ch, nil
}

// templateKind 从模板原文尽力解析 kind（可能含 go template 表达式，取首个 kind: 行的字面值）。
func templateKind(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "kind:") {
			return strings.TrimSpace(strings.TrimPrefix(t, "kind:"))
		}
	}
	return ""
}

// resolveVersion 定位 chart 的指定版本；version 为空时取最新（最新版本记录）。
func (c *controller) resolveVersion(ctx context.Context, id int64, version string) (*model.AppstoreChartVersion, error) {
	chart, err := c.factory.Chart().GetChart(ctx, id)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if chart == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart not found"), http.StatusNotFound)
	}

	if version == "" {
		version = chart.LatestVersion
	}
	if version == "" {
		return nil, apierrors.NewError(fmt.Errorf("chart %s has no available version", chart.Name), http.StatusNotFound)
	}

	cv, err := c.factory.Chart().GetChartVersion(ctx, chart.Id, version)
	if err != nil {
		return nil, apierrors.ErrServerInternal
	}
	if cv == nil {
		return nil, apierrors.NewError(fmt.Errorf("chart %s version %s not found", chart.Name, version), http.StatusNotFound)
	}
	return cv, nil
}

// maxUploadBytes 返回 chart 包大小上限（字节）：MaxUploadMB 未配置或非正时回退默认 32MB（与配置层 SetDefaults 一致）。
func (c *controller) maxUploadBytes() int64 {
	if mb := int64(c.cc.Appstore.MaxUploadMB); mb > 0 {
		return mb << 20
	}
	return defaultMaxUploadMB << 20
}

// readLimited 按 maxUploadBytes 限制读取上传流，超限返回 413。
func (c *controller) readLimited(r io.Reader) ([]byte, error) {
	limit := c.maxUploadBytes()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to read upload: %w", err), http.StatusBadRequest)
	}
	if int64(len(data)) > limit {
		return nil, apierrors.NewError(fmt.Errorf("chart package exceeds max size %d MB", limit>>20), http.StatusRequestEntityTooLarge)
	}
	return data, nil
}

func normalizePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func toChartInfo(chart *model.Chart, versions []model.AppstoreChartVersion) types.ChartInfo {
	spec := types.FormatTime(chart.GmtCreate, chart.GmtModified)
	info := types.ChartInfo{
		Id:            chart.Id,
		Name:          chart.Name,
		Category:      chart.Category,
		Description:   chart.Description,
		Icon:          chart.Icon,
		Source:        chart.Source,
		RepoUrl:       chart.RepoURL,
		LatestVersion: chart.LatestVersion,
		Downloads:     chart.Downloads,
		InstallCount:  chart.InstallCount,
		CreatedBy:     chart.CreatedBy,
		CreatedByName: chart.CreatedByName,
		GmtCreate:     spec.GmtCreate.(string),
		GmtModified:   spec.GmtModified.(string),
	}
	if versions != nil {
		info.Versions = make([]types.ChartVersionInfo, 0, len(versions))
		for _, v := range versions {
			vs := types.FormatTime(v.GmtCreate, v.GmtModified)
			info.Versions = append(info.Versions, types.ChartVersionInfo{
				Version:     v.Version,
				AppVersion:  v.AppVersion,
				Description: v.Description,
				Icon:        v.Icon,
				Digest:      v.Digest,
				Size:        v.Size,
				GmtCreate:   vs.GmtCreate.(string),
			})
		}
	}
	return info
}
