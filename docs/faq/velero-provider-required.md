# FAQ-006 Velero 应用安装失败（BackupStorageLocation/VolumeSnapshotLocation spec.provider Required value）

- **分类**：应用商店 / Velero
- **涉及组件**：Velero Chart 12.2.1 / Velero CRD / 应用商店平台默认配置

## 现象

应用商店安装 Velero（Chart 12.2.1）失败，失败原因：

```text
failed to install chart: 2 errors occurred:
* BackupStorageLocation.velero.io "default" is invalid: spec.provider: Required value
* VolumeSnapshotLocation.velero.io "default" is invalid: spec.provider: Required value
```

应用商店显示「安装失败」，Helm 状态可能为「未知」；集群里可能已有 velero Deployment/Pod 在运行，但不存在 `BackupStorageLocation` / `VolumeSnapshotLocation`。

## 根因

Velero Chart 12.2.1 的默认 values 自带一条默认 BSL 和一条默认 VSL，但 `provider` 和 `bucket` 都是空：

```yaml
backupsEnabled: true
snapshotsEnabled: true
configuration:
  backupStorageLocation:
    - name:
      provider: ""
      bucket: ""
  volumeSnapshotLocation:
    - name:
      provider: ""
```

模板会无条件渲染 `provider: {{ .provider }}`，而 Velero CRD 要求 `spec.provider` 非空，因此 Helm 在这里校验失败。

这不是 Velero 运行时的硬性要求——Velero 本身可以在没有 BSL/VSL 的情况下启动；问题在于 **Chart 默认值不合法**，而应用商店按默认值安装。

## 验证方式

查看 Helm release 状态（可选，应用商店报错已足够）：

```bash
kubectl -n pixiu-system get backupstoragelocation,volumesnapshotlocation
# 预期为空；Velero Pod 可能已存在
```

## 解决方案

### 方案 A：零配置安装（推荐给多用户平台）

在应用商店该 Velero Chart 版本（12.2.1）的「平台默认配置」中设置：

```yaml
configuration:
  backupStorageLocation: []
  volumeSnapshotLocation: []
```

保存后，安装弹窗会自动加载该默认配置，用户不需要展开「配置」或填写 values，直接安装即可。

效果：

- Helm 安装成功，Velero Pod 正常运行；
- 此时没有备份存储，备份功能不可用；
- 应用详情建议显示「已安装 · 备份存储未配置」，并提供「配置备份存储」入口。

> 已失败的 velero release 需要先在应用商店卸载/清理，再重新安装；不要直接重复安装，否则同名 release 冲突。

### 方案 B：安装时直接配置可用备份存储

如果需要安装后即可备份，在平台默认配置或安装弹窗的「配置」中提供完整 values。以 MinIO/S3 兼容存储为例：

```yaml
initContainers:
  - name: velero-plugin-for-aws
    image: swr.cn-north-4.myhuaweicloud.com/pixiu-public/velero/velero-plugin-for-aws:v1.13.1
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins

credentials:
  secretContents:
    cloud: |
      [default]
      aws_access_key_id=xxx
      aws_secret_access_key=xxx

configuration:
  backupStorageLocation:
    - name: default
      provider: aws
      bucket: velero
      default: true
      config:
        region: us-east-1
        s3Url: http://minio.minio.svc:9000
        s3ForcePathStyle: "true"
  volumeSnapshotLocation: [] # 不用原生快照插件时留空
  defaultVolumesToFsBackup: true # 可选：默认用 Kopia 文件级备份

deployNodeAgent: true
```

要点：

- 对象存储 provider 用 `aws`，MinIO 也用它；`s3ForcePathStyle: "true"` 对 MinIO 必须；
- `initContainers` 的 AWS 插件不能省，否则 Velero 不认识 `provider: aws`；
- `deployNodeAgent: true` 才会创建 node-agent，Kopia 文件级备份需要；
- 多用户平台建议把共享的 MinIO/S3 配置写入平台默认配置，用户无需自行填写。

### 方案 C：装好后再接入 provider

方案 A 安装成功后，也可以在后续升级时补充：

1. 升级 Velero values，增加 `initContainers`（AWS 插件）和 `credentials`；
2. 增加 `configuration.backupStorageLocation`，或手动创建 BSL：

   ```yaml
   apiVersion: velero.io/v1
   kind: BackupStorageLocation
   metadata:
     name: default
     namespace: pixiu-system
   spec:
     provider: aws
     objectStorage:
       bucket: velero
     config:
       region: us-east-1
       s3Url: http://minio.minio.svc:9000
       s3ForcePathStyle: "true"
     credential:
       name: velero-credentials
       key: cloud
   ```

3. 插件是 initContainer，必须通过升级/重启 Pod 加载；BSL/VSL 可以随时创建，不需要重装 Velero。

> 如果 BSL 是手动创建的、Helm values 中 `backupStorageLocation: []`，后续升级不会删除它；如果写在 values 里，手动修改可能被升级覆盖。

### 卷快照 / CSI 说明

- Velero 1.18 的 CSI 快照是**内置**的：使用 `configuration.features: EnableCSI` + 带 `velero.io/csi-volumesnapshot-class=true` 标签的 `VolumeSnapshotClass`，**不要**配置 `provider: csi` 的 VolumeSnapshotLocation。
- 只有 AWS EBS/Azure/GCP/vSphere 等原生快照插件才使用 VSL `provider: aws` 等。
- 只用 Kopia 文件级备份时，不需要 VSL，保持 `volumeSnapshotLocation: []` 并设置 `deployNodeAgent: true` 即可。

## 备注 / 风险

- **不要**用占位 `provider: aws` + 空 bucket 绕过校验：安装会成功，但备份不可用。
- 修改 Chart 包不是首选：应用商店的平台默认配置可以一次性覆盖所有安装；只有脱离应用商店直接分发 tgz，或平台默认配置不可用时，才改 Chart 默认 values。
- 应用商店升级的合并优先级是「本次请求 values > 当前 release values > 平台默认配置」；升级时如果只改平台默认配置，可能被当前 release 的旧空列表覆盖，需在升级请求中显式传 values 或卸载重装。
- 多用户场景：平台默认配置是一次性管理员操作，用户无需改动 values；若要求“装完即可备份”，平台需要提供默认对象存储（MinIO preset/共享 S3），否则用户仍需各自配置。
- Velero 没有 BSL 时可以运行，但不能执行备份；应用状态不要只显示绿色「已安装」，应显示「备份存储未配置」。

## 参考

- Velero v1.18 Backup Storage Locations / Volume Snapshot Locations: <https://velero.io/docs/v1.18/locations/>
- Velero CSI Snapshot Support: <https://velero.io/docs/main/csi/>
- 仓库 Chart values: `pixiu-chart-repo/charts/velero-12.2.1.tgz` → `velero/values.yaml`
