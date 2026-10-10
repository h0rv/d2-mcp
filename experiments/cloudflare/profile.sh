#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
export XDG_CONFIG_HOME="$PWD/.wrangler/config"
export WRANGLER_LOG_PATH="$PWD/.wrangler/logs/startup.log"
export WRANGLER_SEND_METRICS=false
# This command internally builds with --dry-run, then profiles local workerd.
npx --no-install wrangler check startup --outfile build/startup.cpuprofile
