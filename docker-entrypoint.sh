#!/bin/sh
# Runs the Go API and the React Router SSR server in one container.
#
#   docker-entrypoint.sh [api flags...]   e.g. -port=8080 -refresh=5m
#
# Arguments go to the Go API (cmd/main.go reads flags, not env). The web
# server reads PORT (default 3000) and API_URL (default
# http://localhost:8080) from the environment.
#
# Signals: tini runs this script with -g, so a SIGTERM sent to the container
# reaches the whole process group — both servers get it directly and shut
# down on their own. SIGINT is turned into a SIGTERM for the group (below).
#
# Liveness: if either server exits, the other is stopped and the script
# exits non-zero, so the container restarts instead of running half an app.
set -u

WEB_SERVER=${WEB_SERVER:-/app/node_modules/.bin/react-router-serve}
WEB_BUILD=${WEB_BUILD:-/app/build/server/index.js}
API_BIN=${API_BIN:-/api}

# Each supervised process reports "<name> <exit status>" on fd 3 when it
# exits. fd 3 is an anonymous FIFO held open read-write here, so writers
# never block and a report sent before we start reading is not lost.
fifo_dir=$(mktemp -d)
mkfifo "$fifo_dir/exits"
exec 3<>"$fifo_dir/exits"
rm -rf "$fifo_dir"

# Background jobs of a non-interactive shell start with SIGINT ignored, so
# the servers never see an INT: translate it into a TERM for the group.
stopping=0
trap 'stopping=1' TERM
trap 'stopping=1; kill -TERM 0 2>/dev/null' INT

run() {
	name=$1
	shift
	(
		# Keep the watcher alive through the group-wide SIGTERM so it can
		# report the exit status; the command itself gets default handling.
		trap ':' TERM INT
		"$@"
		echo "$name $?" >&3
	) &
}

run api "$API_BIN" "$@"
run web "$WEB_SERVER" "$WEB_BUILD"

# `read` returns early (non-zero) when a trapped signal arrives; retry.
until read -r first_name first_rc <&3; do :; done

unexpected=0
if [ "$stopping" -eq 0 ]; then
	unexpected=1
	echo "docker-entrypoint: $first_name exited on its own (status $first_rc); stopping the other server" >&2
	stopping=1
	# The whole group: the remaining server, the watchers and this shell
	# (both of which trap it).
	kill -TERM 0 2>/dev/null
fi

until read -r second_name second_rc <&3; do :; done
echo "docker-entrypoint: $first_name exited $first_rc, $second_name exited $second_rc" >&2

# A server that stopped on its own is a failure even when it exited 0.
if [ "$unexpected" -eq 1 ]; then
	[ "$first_rc" -ne 0 ] && exit "$first_rc"
	exit 1
fi
exit 0
