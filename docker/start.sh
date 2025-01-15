#!/bin/sh
set -o errexit
set -o xtrace

if [[ ! -d "/etc/pixiu" ]]; then
    mkdir -p /etc/pixiu
fi

if [[ -e "/static/config.json" ]]; then
    rm -rf /static/config.json
fi
if [[ -e "/etc/pixiu/config.json" ]]; then
    cp /etc/pixiu/config.json /static/config.json
fi

/app