#!/usr/bin/env bash
# Runs every example for real against the spec repo's mock server.
#   STAMPMITRA_SPEC_DIR=/path/to/stampmitra-developer-openapi ./scripts/run-examples.sh
set -euo pipefail

SPEC_DIR="${STAMPMITRA_SPEC_DIR:-/home/user/stampmitra-developer-openapi}"
NODE="$(command -v node || echo /opt/node22/bin/node)"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${MOCK_PORT:-$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')}"
RPORT="${RECEIVER_PORT:-$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')}"

"$NODE" "$SPEC_DIR/scripts/mock-server.mjs" --port "$PORT" --host 127.0.0.1 >/dev/null 2>&1 &
MOCK_PID=$!
RECV_PID=""
cleanup() { kill "$MOCK_PID" ${RECV_PID:+"$RECV_PID"} 2>/dev/null || true; }
trap cleanup EXIT
for _ in $(seq 1 50); do curl -sf "http://127.0.0.1:$PORT/__mock/health" >/dev/null && break; sleep 0.2; done

export STAMPMITRA_API_KEY="sk_test_exampleKey1234"
export STAMPMITRA_BASE_URL="http://127.0.0.1:$PORT"
cd "$ROOT/examples"

echo "== verify_pan";       go run ./verify_pan ABCDE1234F "Rahul Kumar"
echo "== create_order";     go run ./create_order
echo "== logs_pagination";  go run ./logs_pagination

echo "== webhook_receiver"
export WEBHOOK_SECRET="whsec_test_0123456789abcdef" PORT="$RPORT"
go build -o "$ROOT/examples/.receiver" ./webhook_receiver
"$ROOT/examples/.receiver" >/dev/null 2>"$ROOT/examples/.receiver.log" &
RECV_PID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null "http://127.0.0.1:$RPORT/" && break; sleep 0.1; done
BODY='{"coreOrderId":"core_order_9f3a1c2b8e4d4a5f","coreEventType":"DOCUMENT_READY","occurredAt":"2026-09-25T09:05:00.000Z"}'
TS="$(date +%s)"
SIG="$(printf '%s' "$TS.$BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | awk '{print $NF}')"
post() { curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$RPORT/webhooks/stampmitra" -H "X-Webhook-Signature: $1" -H "X-Webhook-Timestamp: $TS" -H "X-Webhook-Event-Id: evt_1" --data "$BODY"; }
GOOD="$(post "$SIG")"; BAD="$(post "0000")"
echo "valid signature -> HTTP $GOOD (want 204); bad signature -> HTTP $BAD (want 400)"
cat "$ROOT/examples/.receiver.log"
rm -f "$ROOT/examples/.receiver" "$ROOT/examples/.receiver.log"
[ "$GOOD" = 204 ] && [ "$BAD" = 400 ]
