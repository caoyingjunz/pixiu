#!/bin/sh
set -o errexit
set -o xtrace

if [[ ! -d "/etc/pixiu" ]]; then
    mkdir -p /etc/pixiu
fi

# 暂时支持 80 端口
cp /nginx-default-80.conf /etc/nginx/conf.d/default.conf

/app &
nginx -g "daemon off;"
