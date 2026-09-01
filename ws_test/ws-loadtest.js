// k6 WebSocket 压测脚本 —— 两种模式
//   MODE=conn        并发连接数压测：每个 VU 建立 1 条连接，保持整轮不断开
//   MODE=throughput  消息吞吐压测：VUs 加入同一房间并周期性发 chat，统计收/发消息数
//
// 脚本本身不读任何服务器秘密；token 由 gen-tokens.mjs 预先生成到 tokens.txt。
// 每个 VU 用 tokens[__VU-1]，保证 user_id 互不相同，避免被服务端按 user_id 踢下线。
//
// 运行：见 run-k6.sh，或手动：
//   k6 run --env MODE=conn --env CONNS=1000 --env DURATION=60s ws-loadtest.js
//
// 说明：
// - 服务端 WritePump 会把积压的多个 JSON 用 "\n" 拼在同一个 WS 帧里下发，
//   因此收包时必须按 "\n" 拆分后逐个 JSON.parse（与 ws_test/e2e-test.mjs 一致）。
// - BroadcastToRoom 使用非阻塞通道发送，客户端缓冲(256)满时会静默丢包，
//   所以 throughput 模式下收到的消息数 < 理论广播数，是被测出的真实上限。
import ws from "k6/ws";
import { check } from "k6";
import { Counter } from "k6/metrics";
import { SharedArray } from "k6/data";

const tokens = new SharedArray("tokens", function () {
  const txt = open("tokens.txt");
  const arr = txt.split("\n").filter(Boolean);
  if (arr.length === 0) {
    throw new Error("tokens.txt 为空，请先运行: node gen-tokens.mjs <数量>");
  }
  return arr;
});

const MODE = (__ENV.MODE || "conn").trim();
const WS_BASE = __ENV.WS_BASE || "ws://localhost:8080/api/ws";
const PARTY_ID = __ENV.PARTY_ID || "1";
const CONNS = parseInt(__ENV.CONNS || "1000", 10);
const TP_VUS = parseInt(__ENV.TP_VUS || "200", 10);
const DURATION = __ENV.DURATION || "60s";
// 连接保持时长。设得 ≥ 测试时长，避免 VU 在一个 iteration 里反复断开/重连产生 churn，
// 从而让“同时保持的连接数 ≈ VU 数”。默认 10 分钟。
const HOLD_MS = parseInt(__ENV.HOLD_MS || "600000", 10);
const SEND_INTERVAL_MS = parseInt(__ENV.SEND_INTERVAL_MS || "200", 10);

// 指标
const msgsReceived = new Counter("ws_msgs_received");
const msgsSent = new Counter("ws_msgs_sent");
const connsOpened = new Counter("ws_conns_opened");
const connErrors = new Counter("ws_conn_errors");

// ---------- 选项：按 MODE 走不同 executor ----------
const connOptions = {
  scenarios: {
    conn: {
      executor: "constant-vus",
      vus: CONNS,
      duration: DURATION,
      exec: "connRun",
    },
  },
};

const throughputOptions = {
  scenarios: {
    throughput: {
      executor: "constant-vus",
      vus: TP_VUS,
      duration: DURATION,
      exec: "throughputRun",
    },
  },
};

export const options = MODE === "throughput" ? throughputOptions : connOptions;

function pickToken() {
  const t = tokens[__VU - 1];
  if (!t) {
    throw new Error(`token 不足：__VU=${__VU}，但 tokens.txt 只有 ${tokens.length} 个。请用更大的数量重新生成。`);
  }
  return t;
}

// ---------- 模式一：并发连接数 ----------
export function connRun() {
  let opened = false;
  const url = `${WS_BASE}?token=${pickToken()}`;
  ws.connect(url, { handshakeTimeout: "30s" }, function (socket) {
    socket.on("open", function () {
      opened = true;
      connsOpened.add(1);
      // 保持连接：等 HOLD_MS 后主动关闭；若 HOLD_MS ≥ 测试时长则整轮只开 1 条连接
      socket.setTimeout(function () {
        socket.close();
      }, HOLD_MS);
    });

    socket.on("message", function (data) {
      // 连接模式顺便解析，确认能收到数据（join 不发的场景可能为空）
      for (const line of data.toString().split("\n")) {
        if (!line.trim()) continue;
        try {
          const m = JSON.parse(line);
          if (m.type === "chat_history" || m.type === "chat") msgsReceived.add(1);
        } catch (e) { /* 忽略非 JSON */ }
      }
    });

    socket.on("ping", function () {});
    socket.on("pong", function () {});
    socket.on("error", function (e) { connErrors.add(1); });
    socket.on("close", function () {});
  });

  check({}, { "连接建立(ws open)": () => opened });
}

// ---------- 模式二：消息吞吐 ----------
export function throughputRun() {
  const url = `${WS_BASE}?token=${pickToken()}`;
  ws.connect(url, { handshakeTimeout: "30s" }, function (socket) {
    socket.on("open", function () {
      connsOpened.add(1);
      // 加入同一房间（广播天然会算上自己，作为回显）
      socket.send(JSON.stringify({ type: "join_room", partyId: PARTY_ID }));

      // 周期性发 chat 消息
      socket.setInterval(function () {
        socket.send(JSON.stringify({
          type: "chat",
          partyId: PARTY_ID,
          content: "k6-load",
          time: new Date().toTimeString().slice(0, 5),
        }));
        msgsSent.add(1);
      }, SEND_INTERVAL_MS);

      socket.setTimeout(function () {
        socket.close();
      }, HOLD_MS);
    });

    socket.on("message", function (data) {
      // 服务端把多条 JSON 用 "\n" 拼在一个 WS 帧里，需拆分解析
      for (const line of data.toString().split("\n")) {
        if (!line.trim()) continue;
        try {
          const m = JSON.parse(line);
          if (m.type === "chat") msgsReceived.add(1);
        } catch (e) { /* 忽略非 JSON */ }
      }
    });

    socket.on("ping", function () {});
    socket.on("pong", function () {});
    socket.on("error", function (e) { connErrors.add(1); });
    socket.on("close", function () {});
  });
}
