#!/bin/bash
function update() {
    docker pull harbor.cloud.pixiuio.com/pixiuio/pixiu-aio:latest
    docker rm -f pixiu-aio
    sleep 3
    docker run -d --net host --restart=always --privileged=true -v /etc/pixiu:/etc/pixiu -v /var/run/docker.sock:/var/run/docker.sock --name pixiu-aio harbor.cloud.pixiuio.com/pixiuio/pixiu-aio:latest
}

update
