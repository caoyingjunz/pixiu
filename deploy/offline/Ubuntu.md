### 将 3 个离线包移到 data目录里，进行自动加载
```bash
root@VM-32-17-ubuntu:~# mv pixiu-images-amd64-v1.31.6.tar.gz  pixiu-packages-ubuntu-24.04-amd64-v1.31.6.tar.gz pixiu-server-images-amd64.tar.gz data/
```
正常回显
```bash
检测到新离线包 data/pixiu-images-amd64-v1.31.6.tar.gz，正在加载 ...
  + 10.206.32.17:5000/calico-cni:v3.31.3
  + 10.206.32.17:5000/calico-kube-controllers:v3.31.3
  + 10.206.32.17:5000/calico-node:v3.31.3
  + 10.206.32.17:5000/calico-typha:v3.31.3
  + 10.206.32.17:5000/flannel-cni-plugin:v1.4.1-flannel1
  + 10.206.32.17:5000/flannel:v0.25.4
  + 10.206.32.17:5000/metrics-scraper-nginx:1.25-pixiu
....
```

### 测试镜像仓库和源仓库可用性
```bash 
# ubuntu (ip根据实际情况调整)
# 清理 /etc/apt/sources.list.d/ 下面的其他源
rm -rf /etc/apt/sources.list.d/*
echo 'deb [trusted=yes] http://10.206.32.8:8080/deb ./' > /etc/apt/sources.list.d/pixiu.list
apt update
```

#### 安装docker
```bash
apt-get install docker-ce

# 配置 insecure-registries
cat > /etc/docker/daemon.json <<'EOF'
{
  "insecure-registries": ["0.0.0.0/0"]
}
EOF

systemctl restart docker
systemctl enable docker
```
![img_2.png](img_2.png)
