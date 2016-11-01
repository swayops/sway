#!/bin/bash

pushd $GOPATH/src/github.com/swayops/swayops
go get -u -ldflags "-X github.com/swayops/sway/server.gitBuild=$(git describe --always --abbrev=16)" || exit 1
popd
