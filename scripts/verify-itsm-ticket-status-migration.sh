#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "[verify] 运行 ITSM 状态迁移回归样本..."
go test ./internal/app/itsm/bootstrap -run '^TestMigrateTicketStatusModelMapsLegacyStatusToNewStatusAndOutcome$' -count=1

echo "[verify] 迁移回归样本通过。"
