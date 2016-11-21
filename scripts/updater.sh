#!/bin/bash

export NODE_ENV=production

SWAY_USER="sway"

function asSwayUser() {
	if [ "$(id -un)" == "$SWAY_USER" ]; then
		if [ ! -d "$GOPATH" ]; then
			die "GOPATH ($GOPATH) does not exist."
		fi
		$1
	else
		echo "rerunning as $SWAY_USER..."
		sudo -u $SWAY_USER -i $(realpath $0) $1
	fi
}

function die {
	if [ "$@" != "" ]; then
		echo $@ > /dev/stderr
	fi
	exit 1
}

function update-sway() {
	rm $GOPATH/bin/sway
	go get -u -ldflags "-X github.com/swayops/sway/server.gitBuild=$(git describe --always --abbrev=16)" || exit 1
}

function update-dash() {
	pushd $GOPATH/src/github.com/swayops/dashboard || die ""
	git pull || exit 1
	npm install || exit 1
	popd > /dev/null
}

function update-infApp() {
	pushd $GOPATH/src/github.com/swayops/influencerApp || die ""
	git pull || exit 1
	npm install || exit 1
	popd > /dev/null
}

function update-all() {
	update-sway && update-dash && update-infApp
}

case "$1" in
	dash|dashboard|update-dash) echo "updating dashboard"; asSwayUser update-dash ;;
	inf|infApp|influencerApp|update-infApp) echo "updating influencerApp"; asSwayUser update-infApp ;;
	sway|up|update|update-sway) echo "updating sway"; asSwayUser  update-sway ;;
	a|all|upa|update-all) echo 'updating all'; asSwayUser update-all;;
	r|restart) echo 'restarting sway'; sudo 'systemctl restart sway && sleep 2s && systemctl status -l sway';;
	start) echo 'starting sway'; sudo 'systemctl start sway && sleep 2s && systemctl status -l sway';;
	stop) echo 'stoping sway'; sudo 'systemctl stop sway';;
	*) echo "$0 [dash|inf|sway|all]" ;;
esac
