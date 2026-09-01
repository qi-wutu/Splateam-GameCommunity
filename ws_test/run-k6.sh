#!/bin/bash
# WebSocket 压测一键运行脚本
# 前置条件：
#   - 已安装 k6 与 node
#   - 服务器已启动（go run . 或 ./server），且 MySQL/Redis 可用；若只测连接可在服务器加 --env 关闭聊天(可选)
# 用法（在本 ws_test 目录下）：
#   MODE=conn CONNS=5000 DURATION=60s ./run-k6.sh
#   MODE=throughput TP_VUS=200 DURATION=60s PARTY_ID=1 ./run-k6.sh
set -euo pipefail
cd "$(dirname "$0")"

MODE="${MODE:-conn}"
CONNS="${CONNS:-1000}"
TP_VUS="${TP_VUS:-200}"
DURATION="${DURATION:-60s}"
SEND_INTERVAL_MS="${SEND_INTERVAL_MS:-200}"
WS_BASE="${WS_BASE:-ws://localhost:8080/api/ws}"
PARTY_ID="${PARTY_ID:-1}"
# 注意：当前服务端 utils.InitJWT() 从未被调用，jwtSecret 实际为空串。
# 所以生成的 token 必须用空 secret 签名才会被服务端接受。若你修复了该 bug（在 main 里调用 InitJWT），
# 请把 secret 换成 config/.env 里的 JWT_SECRET 值。
JWT_SECRET="${JWT_SECRET:-}"
HOLD_MS="${HOLD_MS:-600000}"

if ! command -v k6 >/dev/null 2>&1; then
  echo "❌ 未找到 k6，请先安装: https://k6.io/docs/getting-started/installation/"
  exit 1
fi
if ! command -v node >/dev/null 2>&1; then
  echo "❌ 未找到 node，请先安装 Node.js"
  exit 1
fi

TOKENS_NEEDED=$(( MODE == "throughput" ? TP_VUS : CONNS ))

echo ">>> 1/3 生成 ${TOKENS_NEEDED} 个独立 JWT ($JWT_SECRET 前缀 load)..."
node gen-tokens.mjs "${TOKENS_NEEDED}" "${JWT_SECRET}"

echo ">>> 2/3 调整文件句柄上限（尽力而为，当前: $(ulimit -n)）..."
ulimit -n 200000 2>/dev/null || echo "    ⚠️ 无法提升 ulimit -n，连接数可能受客户端句柄上限限制"

echo ">>> 3/3 运行 k6: MODE=${MODE} ${MODE==conn?CONNS:TP_VUS} 连接 @ DURATION=${DURATION}"
echo "    端点: ${WS_BASE}  party: ${PARTY_ID}"
if [ "$MODE" = "throughput" ]; then
  echo "    （消息吞吐模式：所有 VU 加入 room ${PARTY_ID}，广播含回显，收到数会放大，为真实广播上限）"
else
  echo "    （连接模式：服务器日志里 '在线: N' 的最大值 = 同时保持连接数，即你要的数字）"
fi

k6 run \
  --env MODE="${MODE}" \
  --env WS_BASE="${WS_BASE}" \
  --env PARTY_ID="${PARTY_ID}" \
  --env CONNS="${CONNS}" \
  --env TP_VUS="${TP_VUS}" \
  --env DURATION="${DURATION}" \
  --env SEND_INTERVAL_MS="${SEND_INTERVAL_MS}" \
  --env HOLD_MS="${HOLD_MS}" \
  ws-loadtest.js

echo ""
echo ">>> 若测连接数，后端日志峰值：grep -E '在线: ' <server日志> | grep -oE '[0-9]+' | sort -n | tail -1"
