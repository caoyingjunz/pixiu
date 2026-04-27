# 使用 Ubuntu 作为基础镜像
FROM ubuntu:22.04

ARG K8S_VERSION
ARG HELM_VERSION

# 安装vim
RUN apt-get update && apt-get install -y vim

# 安装必要的软件包
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y curl bash-completion

# 安装 kubectl
RUN ARCH=$(uname -m|sed 's|x86_64|amd64|'|sed 's|aarch64|arm64|') && \
    curl -LO "https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${ARCH}/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm -rf kubectl

# 安装 helm
RUN ARCH=$(uname -m|sed 's|x86_64|amd64|'|sed 's|aarch64|arm64|') && \
    curl -LO "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" && \
    tar -zxvf helm-${HELM_VERSION}-linux-${ARCH}.tar.gz && \
    rm -rf helm-${HELM_VERSION}-linux-${ARCH}.tar.gz && \
    mv linux-${ARCH}/helm /usr/local/bin/helm && \
    rm -rf linux-${ARCH}

# 配置 bash 补全和自定义的 PS1 提示符
RUN echo 'source /etc/profile.d/bash_completion.sh' >> /root/.bashrc \
    && echo 'source <(kubectl completion bash)' >> /root/.bashrc \
    && echo 'source <(helm completion bash)' >> /root/.bashrc \
    && echo "PS1='[\[\033[0;34m\]\u\[\033[0;37m\]@\[\033[0;35m\]\h\[\033[0;33m\] \w\[\033[0;37m\]]\[\033[0;31m\]\$\[\033[00m\] '" >> /root/.bashrc

# 设置工作目录
WORKDIR /root

# 设置环境变量等（可选）
ENV PATH="/usr/local/bin:${PATH}"

# 启动 bash 终端
CMD ["sleep", "infinity"]
