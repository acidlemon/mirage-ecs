#!/bin/bash
# Starts, stops or restarts mirage-ecs for the e2e test.
#
#   mirage.sh start|stop|restart
#
# test/e2e/run.sh and the runbooks call this. It requires the environment
# variables which run.sh sets.
set -euo pipefail

cd "$(dirname "$0")/../.."

tmp="${MIRAGE_E2E_TMP:?}"
pidfile="$tmp/mirage-ecs.pid"

start() {
  nohup ./mirage-ecs -conf "$tmp/config.yaml" >> "$tmp/mirage-ecs.log" 2>&1 < /dev/null &
  echo $! > "$pidfile"
  for _ in $(seq 1 30); do
    if curl -fs -o /dev/null "$MIRAGE_E2E_ENDPOINT/"; then
      return
    fi
    sleep 1
  done
  echo "mirage-ecs did not start" >&2
  exit 1
}

stop() {
  if [[ ! -f "$pidfile" ]]; then
    return
  fi
  pid="$(cat "$pidfile")"
  rm -f "$pidfile"
  kill "$pid" 2>/dev/null || return 0
  for _ in $(seq 1 30); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return
    fi
    sleep 1
  done
  kill -KILL "$pid" 2>/dev/null || true
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  restart) stop && start ;;
  *)
    echo "usage: $0 start|stop|restart" >&2
    exit 1
    ;;
esac
