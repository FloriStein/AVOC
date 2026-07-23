// k6 Load Test — Control Loop ACK Latency (ADR-006/010)
// Threshold: p(99) < 100ms (CI Build-Fail on violation)
// Run: make test-k6   (starts test stack automatically)
// Manual: docker run --rm --network host grafana/k6 run - < tests/performance/latency.js

import http from 'k6/http'
import ws from 'k6/ws'
import { check, sleep } from 'k6'
import { Trend, Rate } from 'k6/metrics'

const ackLatency = new Trend('ack_latency_ms', true)
const successRate = new Rate('ack_success_rate')

// DRIFT-M19: real WS ACK roundtrip needs a single, stateful ACTIVE_OPERATOR session against one
// vehicle (only one operator can hold ACTIVE_OPERATOR per vehicle at a time — see session/manager.go).
// Concurrent VUs would just fight each other for that one role and measure lock contention, not ACK
// latency, so this is 1 VU looping sequentially — mirrors BenchmarkControlACKRoundtrip's single
// long-lived connection in tests/performance/latency_test.go, just re-opened once per iteration
// instead of held open for all of them (k6/ws's ws.connect() blocks for the connection's lifetime).
export const options = {
  vus: 1,
  duration: '30s',
  thresholds: {
    // ADR-010: <100ms ACK-Roundtrip — CI Build-Fail on violation
    'ack_latency_ms': ['p(99)<100'],
    'ack_success_rate': ['rate>0.99'],
  },
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:18080'
const AUTH_URL = __ENV.AUTH_URL || 'http://localhost:18081'
const WS_URL = BASE_URL.replace(/^http/, 'ws')

// Minimal Protobuf ControlCommand: field 2 (type=DEADMAN_HOLD=6) as varint — mirrors
// cmdDeadmanHold in tests/performance/latency_test.go's BenchmarkControlACKRoundtrip.
const CMD_DEADMAN_HOLD = new Uint8Array([0x10, 0x06]).buffer

export function setup() {
  // Login as the seeded admin account (ADMIN_PASSWORD=admin_test_secret in docker-compose.test.yml)
  // — there is no "accept any" auth anymore since ADR-024.
  const loginResp = http.post(
    `${AUTH_URL}/auth/operator/login`,
    JSON.stringify({ username: 'admin', password: 'admin_test_secret' }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  check(loginResp, { 'login ok': (r) => r.status === 200 })
  const token = loginResp.json('token')

  // vehicle-int-mock is the only vehicle with a real WS connection in the test stack.
  const sessionResp = http.post(
    `${BASE_URL}/session/start`,
    JSON.stringify({
      vehicle_id: 'vehicle-int-mock',
      operator_id: 'admin',
      operator_role: 'ACTIVE_OPERATOR',
    }),
    { headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` } },
  )
  check(sessionResp, { 'session/start ok': (r) => r.status === 200 })
  const sessionId = sessionResp.json('session_id')

  return { token, sessionId }
}

export default function (data) {
  const { token, sessionId } = data
  const wsUrl = `${WS_URL}/ws?token=${token}&session_id=${sessionId}`

  const res = ws.connect(wsUrl, {}, function (socket) {
    let t0 = 0
    let acked = false

    socket.on('open', function () {
      t0 = Date.now()
      socket.sendBinary(CMD_DEADMAN_HOLD)
    })

    // Server writes the ACK back as a binary Protobuf ControlAck (websocket.go processWSMessage).
    socket.on('binaryMessage', function () {
      ackLatency.add(Date.now() - t0)
      successRate.add(true)
      acked = true
      socket.close()
    })

    // 'close' is the single place that records a failed outcome (fires after 'error' and after
    // the safeguard timeout's forced close too, so success/failure is never double-counted).
    socket.on('close', function () {
      if (!acked) {
        successRate.add(false)
      }
    })

    socket.on('error', function () {})

    // Safeguard so a dropped ACK doesn't hang the VU forever.
    socket.setTimeout(function () {
      if (!acked) {
        socket.close()
      }
    }, 5000)
  })

  check(res, { 'ws handshake status 101': (r) => r && r.status === 101 })

  sleep(0.05)
}

export function teardown(data) {
  http.post(
    `${BASE_URL}/session/end`,
    JSON.stringify({ session_id: data.sessionId }),
    { headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` } },
  )
}
