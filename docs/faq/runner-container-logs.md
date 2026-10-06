# FAQ-004 runner 任务容器日志查看（nerdctl logs 报错）

- **分类**：部署与运行时
- **涉及组件**：部署计划执行（初始化部署环境 / 部署 Master / 部署 Node / 部署基础组件）

## 现象

在 containerd 宿主上查看部署任务容器日志时，`nerdctl logs` 直接报错：

```bash
nerdctl logs bootstrap-servers-20220801
# FATA[0000] invalid LogViewOptions provided (... Namespace:"" ...):
# log viewing options require a ContainerID and Namespace
```

伴随两个容易误判的细节：

1. `nerdctl ps` 中该容器 ID 列显示为任务名的截断（如 `bootstrap-se…`，完整值为 `bootstrap-servers-20220801`），NAMES 列为空
2. 追加 `-n default` 后报错不变

## 根因

部署任务的 runner 容器**不是用 `nerdctl run` 创建的**，而是由 pixiu 经 containerd API 直接创建：

- 容器 ID 即容器名，格式为 `<action>-<planId>`（如 `bootstrap-servers-20220801`；尾部数字是**部署计划 ID，不是日期**）
- 未携带 nerdctl 的容器元数据（名称、日志路径等），因此 `nerdctl ps` 的 NAMES 列为空；`nerdctl logs` 仅支持其自身创建、带有所需元数据的容器，解析出的命名空间为空串后直接报错
- 该报错是预期行为，不是部署故障

## 验证方式

1. `nerdctl ps --no-trunc` → 容器 ID 为 `<action>-<planId>` 全名
2. `ctr -n default containers info <容器名>` → 可见 pixiu 写入的标签（如 `pixiuName=<容器名>`），且无 nerdctl 元数据
3. 对照组：nerdctl 创建的容器（如 `pixiu`、`mariadb`）执行 `nerdctl -n default logs --tail 5 pixiu` 正常

## 解决方案

查看 runner 任务日志的正确方式（任选其一）：

### 方案 A：宿主机日志文件（推荐）

```bash
tail -f /etc/pixiu/runner-logs/<容器名>.log
# 例：tail -f /etc/pixiu/runner-logs/bootstrap-servers-20220801.log
```

容器 stdout/stderr 全程落盘到该文件，路径固定不可配置；**每次任务重新执行会清空重写**。

### 方案 B：平台任务日志

在部署计划的执行详情/任务日志入口查看（Agent 模式执行时，日志会整份上报服务端）。

其他不影响运行的排查手段：`nerdctl exec -it <容器名> sh`、`nerdctl inspect <容器名>`。

## 备注 / 风险

- 容器执行结束后会保留（供事后排查），下一次同名任务执行时自动清理；因此残留的「运行中」容器可能对应已结束的任务
- 任务执行期间若 pixiu 服务重启，日志文件可能停止增长（写入链路随进程退出），以已落盘内容为准
- docker 运行时无此问题（`docker logs <容器名>` 可直接查看）
- 不要尝试给容器手工补 nerdctl 元数据标签来修复 `nerdctl logs` —— 该路径不受支持，日志也不在 nerdctl 的日志体系内
