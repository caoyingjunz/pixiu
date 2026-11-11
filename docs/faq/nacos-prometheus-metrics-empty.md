# FAQ-008 Nacos 实例概览 CPU、内存等图表无数据（未暴露 Prometheus）

- **分类**：中间件 / Nacos
- **涉及组件**：Nacos 实例概览 / 主服务 Actuator

## 现象

实例已连通，版本和命名空间正常，但 CPU、负载、服务内存、线程、GC 图表为空。3.x 更常见；2.x 的 CPU 和负载通常仍有数据。

## 根因

这些图表读的是主服务 Actuator：`/nacos/actuator/prometheus`。官方 `conf/application.properties` 默认注释了：

```properties
#management.endpoints.web.exposure.include=prometheus
```

所以该接口返回 404。Pixiu 不能在页面上打开，配置只在主进程启动时生效。控制台端口（常见 8080）没有这些指标，端口以 `server_port` 或集群节点为准。

3.x 的 `/nacos/v1/ns/operator/metrics` 已不再返回 `cpu`、`load`、`mem`。2.x 仍返回，因此 CPU 和负载不用改这项；服务内存、线程、GC 两个版本都要 Actuator。

## 验证方式

```bash
curl -sS "http://<host>:<server_port>/nacos/actuator"
curl -sS -o /dev/null -w "%{http_code}\n" "http://<host>:<server_port>/nacos/actuator/prometheus"
```

只暴露 `health` 且 Prometheus 返回 404，即是本问题。返回 403 时，用数据源账号换 `accessToken` 后再访问。

## 解决方案

在主服务（不是只改控制台）的 `conf/application.properties` 中取消注释或新增：

```properties
management.endpoints.web.exposure.include=prometheus
```

已有其他端点时保留，例如 `health,prometheus`。然后重启主服务，确认 `/nacos/actuator/prometheus` 返回 200，再刷新实例概览。

## 备注 / 风险

- 不要设成 `*`。只暴露 `prometheus` 会盖掉默认的 `health`，探活需要它时写成 `health,prometheus`。
- 只在内网开放。未填账号或账号无权限时接口 403，图表仍为空。
- 集群里每台主服务都要改并重启。
