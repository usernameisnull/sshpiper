#!/usr/bin/env bash
set -euo pipefail

docker build . \
--build-arg http_proxy=http://172.24.80.1:7890 \
--build-arg https_proxy=http://172.24.80.1:7890 \
-f Dockerfile.proto-gen -t registry.cn-hangzhou.aliyuncs.com/mabing/sshpiper-ci:v0.1