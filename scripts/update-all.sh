#!/bin/bash

export NODE_ENV=production
for repo in $GOPATH/src/github.com/swayops/{dashboard,influencerApp}; do
	pushd $repo
	git pull || exit 1
	npm install || exit 1
	popd
done

go get -u github.com/swayops/sway || exit 1
go install -ldflags "-X github.com/swayops/sway/server.gitBuild=$(git describe --always --abbrev=16)" || exit 1
