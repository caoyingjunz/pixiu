/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (phe "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

const GlobalsTemplate = `# Render below by Pixiu
#####################
# kubernetes options
#####################
# Enable an high availability kubernetes cluster.
#enable_kubernetes_ha: "no"

kube_release: 1.23.6

cluster_cidr: "172.30.0.0/16"
service_cidr: "10.254.0.0/16"

#Network interface is optional, the default vaule
#is eth0.
network_interface: "eth0"

# This should be a VIP, an unused IP on your network that will float between
# the hosts running keepalived for high-availability.
#kube_vip_address: ""

# Enable haproxy and keepalived
# This configuration is usually enabled when self-created VMs require high availability.
#enable_haproxy: "no"

# Listen port for kubernetes.
# 启用 haproxy + keepalived 时, 监听端口推荐使用 8443
#kube_vip_port: 6443

# Kubernetes network cni options
#enable_calico: "no"

# kubernetes 镜像仓库地址，默认阿里云，用户可根据实际情况配置
# 可使用 pixiu 社区镜像仓库：docker.io/pixiuio
image_repository: "registry.cn-hangzhou.aliyuncs.com/google_containers"
`
