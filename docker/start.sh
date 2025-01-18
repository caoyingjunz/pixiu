#!/bin/sh
set -o errexit
set -o xtrace

if [[ ! -d "/etc/pixiu" ]]; then
    mkdir -p /etc/pixiu
fi

if [[ -e "/static/config.json" ]]; then
    rm -rf /static/config.json
fi

URL=$(grep "url" /etc/pixiu/config.yaml | awk -F'url:' '{print $2}' | tr -d '[:space:]')
if [[  -z "$URL" ]]; then
    URL="http://localhost:8080"
fi
cat >/static/config.json << EOF
{
    "url": "${URL}"
}
EOF

/app
