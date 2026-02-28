#!/usr/bin/env bash
set -euo pipefail

# -w, --workdir string                   Working directory inside the container
docker run --rm -it -v ./libplugin:/go/src/sshpiper/libplugin \
-w /go/src/sshpiper/libplugin registry.cn-hangzhou.aliyuncs.com/mabing/sshpiper-ci:v0.1  \
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative plugin.proto