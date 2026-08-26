# 自定义安装包仓库配置

创建 Kubernetes 部署计划时，若节点处于**离线或内网环境**，无法访问公网 apt/yum 源，可启用「自定义源」，让部署过程从私有软件源安装 docker、kubelet 等系统依赖包。

> **说明**：自定义源用于**操作系统软件包**（rpm/deb）；容器**镜像仓库**请在部署配置中单独填写 `image_repository`（Kubernetes 镜像仓库地址）。两者通常指向同一台离线 builder 机器，但配置项不同。

## 前置条件

1. 已按 [离线部署](../../deploy/offline/README.md) 启动 `builder`，软件源与镜像仓库可用。
2. 已在目标节点上验证源可访问（参见 [Ubuntu 示例](../../deploy/offline/Ubuntu.md) 或 [麒麟 V10 示例](../../deploy/offline/KylinV10.md)）。

builder 就绪后，日志中会出现类似输出：

```text
软件源已启动: http://10.206.32.17:8080 （rpm=0 deb=0）
Registry:  10.206.32.17:5000
```

请将下文示例中的 `<仓库IP>` 替换为 builder 所在节点 IP。

## 页面配置

在 Pixiu 控制台 **创建部署计划 → 组件配置** 中：

1. 勾选 **启用自定义源**（对应 `custom_repo.enable = true`）
2. 在 **自定义源内容** 文本框中粘贴下方对应发行版的配置（对应 `custom_repo.content`）
3. 同时将 **Kubernetes 镜像仓库** 设为私有 registry，例如 `10.206.32.17:5000`

启用后，Pixiu 会在部署渲染目录写入 `pixiu` 文件，并在 `globals.yml` 中设置 `enable_custom_repo: "yes"`，由 kubez-ansible 在节点上应用该源配置。

## 配置示例

### Ubuntu / Debian（apt）

将 `<仓库IP>` 替换为 builder 地址：

```text
deb [trusted=yes] http://<仓库IP>:8080/deb ./
```

**完整 JSON 示例**（部署计划 `config.component` 片段）：

```json
{
  "custom_repo": {
    "enable": true,
    "content": "deb [trusted=yes] http://10.206.32.17:8080/deb ./"
  }
}
```

节点上手动验证：

```bash
rm -rf /etc/apt/sources.list.d/*
echo 'deb [trusted=yes] http://10.206.32.17:8080/deb ./' > /etc/apt/sources.list.d/pixiu.list
apt update
```

### CentOS / Rocky / OpenEuler / 麒麟 V10（yum）

将 `<仓库IP>` 替换为 builder 地址：

```ini
[pixiu]
name=Pixiu
baseurl=http://<仓库IP>:8080/rpm
enabled=1
gpgcheck=0
```

**完整 JSON 示例**：

```json
{
  "custom_repo": {
    "enable": true,
    "content": "[pixiu]\nname=Pixiu\nbaseurl=http://10.206.32.17:8080/rpm\nenabled=1\ngpgcheck=0"
  }
}
```

节点上手动验证：

```bash
cat > /etc/yum.repos.d/pixiu.repo <<'EOF'
[pixiu]
name=Pixiu
baseurl=http://10.206.32.17:8080/rpm
enabled=1
gpgcheck=0
EOF
yum makecache
```

> **注意（麒麟/CentOS 系）**：部分环境 `yum update` 后会预装 `docker-runc`，可能与后续安装的 docker-ce 冲突，需先执行 `yum remove docker-runc`。详见 [KylinV10 离线说明](../../deploy/offline/KylinV10.md)。

## 与镜像仓库配合使用

离线场景下，部署计划通常需同时配置：

| 配置项 | 作用 | 示例 |
|--------|------|------|
| `kubernetes.image_repository` | 拉取 K8s 及 CNI 等容器镜像 | `10.206.32.17:5000` |
| `custom_repo` | 节点安装 docker、依赖包等系统软件 | 见上文 apt/yum 示例 |
| Runner 镜像 | 部署驱动容器 | `10.206.32.17:5000/pixiu/kubez-ansible:v3.0.4` |

离线部署完整流程参见 [deploy/offline/README.md](../../deploy/offline/README.md#集群部署)。

## 常见问题

**Q：填了自定义源但部署仍访问公网？**

确认已勾选「启用自定义源」，且 `content` 中 IP/端口与 builder 日志一致；部署前在节点上按上文命令验证 `apt update` 或 `yum makecache` 是否成功。

**Q：`content` 里要写完整 repo 文件路径吗？**

不需要。只需填写源定义正文（apt 一行或 yum 的 `[pixiu]` 段），Pixiu 会将其写入部署目录下的 `pixiu` 文件，由 ansible 下发到目标节点。

**Q：联机部署也需要吗？**

不需要。节点能访问公网时，保持 `custom_repo.enable = false`（默认）即可。
