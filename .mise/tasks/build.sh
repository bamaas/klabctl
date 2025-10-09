#!/usr/bin/env bash

set -euo pipefail

cd cmd/klabctl
go build -o ${ROOT_DIR}/bin/klabctl
chmod +x ${ROOT_DIR}/bin/klabctl