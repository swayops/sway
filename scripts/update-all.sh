#!/bin/bash

export NODE_ENV=production

pushd $GOPATH/src/github.com/swayops/sway
rm $GOPATH/bin/sway
go get -u -ldflags "-X github.com/swayops/sway/server.gitBuild=$(git describe --always --abbrev=16)" || exit 1
popd

for repo in $GOPATH/src/github.com/swayops/{dashboard,influencerApp}; do
	pushd $repo
	git pull || exit 1
	npm install || exit 1
	popd
done

