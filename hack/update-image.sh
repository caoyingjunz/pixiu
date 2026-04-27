#!/bin/bash
function update() {
    docker ccr.ccs.tencentyun.com/pixiucloud/pixiu-aio:latest
    docker rm -f pixiu-aio
    sleep 3
    docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu-aio ccr.ccs.tencentyun.com/pixiucloud/pixiu-aio:latest
}

update
