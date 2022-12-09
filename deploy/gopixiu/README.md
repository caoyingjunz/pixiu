# Gopixiu

Gopixiu是貔貅的服务端, 貔貅是一个广泛使用的、具有web-ui的、实现多集群管理的、非常好用的kubernetes(k8s)容器管理平台.

本 chart 使用 [Helm](https://helm.sh) 包管理器实现在[Kubernetes](https://kubernetes.io) (k8s)集群上部署gopixiu

## 安装Chart

注意: 暂时只支持helm3

- 通过源码安装(本示例默认源码安装)

  ```
  helm install [RELEASE_NAME] gopixiu/
  ```

- 修改默认参数后, 打包上传到repos后, 例如https://artifacthub.io/, 再行安装

  ```
  helm repo add gopixiu https://xxxx.xxx.xx/gopixiu
  helm repo update
  helm install [RELEASE_NAME] gopixiu/gopixiu
  ```

## 卸载Chart

通过以下命令卸载:

```console
helm delete gopixiu
```

## 更新Chart

```
helm upgrade [RELEASE_NAME] [CHART] --install
```

## 配置

- 详情请参考: [values.yaml](./values.yaml)

- 自定义参数:

  ```
  helm install gopixiu gopixiu/ --set=service.externalPort=8080,resources.limits.cpu=300m
  ```