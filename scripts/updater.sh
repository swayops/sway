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
	echo "updating sway"
	pushd $GOPATH/src/github.com/swayops/sway > /dev/null
	rm $GOPATH/bin/sway &>/dev/null
	git pull || die
	go get -v -ldflags "-X github.com/swayops/sway/server.gitBuild=$(git describe --always --abbrev=16)" || die
	popd > /dev/null
}

function update-dash() {
	echo "updating dashboard"
	pushd $GOPATH/src/github.com/swayops/dashboard > /dev/null || die
	git pull || exit 1
	npm install || exit 1
	popd > /dev/null
}

function update-infApp() {
	echo "updating influencer app"
	pushd $GOPATH/src/github.com/swayops/influencerApp > /dev/null || die
	git pull || exit 1
	npm install || exit 1
	popd > /dev/null
}

function update-all() {
	echo "updating all"
	update-sway && update-dash && update-infApp
}

case "$1" in
	dash|dashboard|update-dash) asSwayUser update-dash ;;

	inf|infApp|influencerApp|update-infApp) asSwayUser update-infApp ;;

	sway|up|update|update-sway) asSwayUser  update-sway ;;

	a|all|upa|update-all) asSwayUser update-all;;

	r|restart) echo 'restarting sway'; sudo -i /bin/bash -c 'systemctl restart sway && sleep 2s && systemctl status -l sway';;

	start) echo 'starting sway'; sudo -i /bin/bash -c 'systemctl start sway && sleep 2s && systemctl status -l sway';;

	stop) echo 'stoping sway'; sudo -i /bin/bash -c 'systemctl stop sway';;

	st|status) sudo -i /bin/bash -c 'systemctl status -l sway';;

	*) echo "$0 [ dash | inf | sway | all | restart | start | stop | status ]" ;;
esac
