import { WebSocket } from "ws";
import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const BASE = "http://localhost:8080/api";
const TOKEN = readFileSync(join(__dirname, "e2e_token.txt"), "utf8");
const PARTY_ID = readFileSync(join(__dirname, "e2e_party.txt"), "utf8");
const WS_URL = "ws://localhost:8080/api/ws";

if (!TOKEN || !PARTY_ID) {
  console.error("❌ 无法读取 e2e_token.txt 或 e2e_party.txt");
  process.exit(1);
}

let totalTests = 0;
let passedTests = 0;

function assert(name, ok) {
  totalTests++;
  if (ok) passedTests++;
  console.log(`   ${ok ? "✅" : "❌"} ${name}`);
}

console.log("=== 🧪 WebSocket E2E 测试 ===\n");
console.log(`Party ID: ${PARTY_ID}`);

// ---- 测试 1: HTTP API 在线状态 ----
console.log("\n1️⃣  HTTP API 测试");
const onlineRes = await fetch(`${BASE}/user/me`, {
  headers: { Authorization: `Bearer ${TOKEN}` },
});
const onlineData = await onlineRes.json();
assert("获取当前用户信息", onlineRes.ok && onlineData.userName === "E2EUser");

// ---- 测试 2: WebSocket 连接 + 加入房间 + 聊天 ----
console.log("\n2️⃣  WebSocket 单客户端测试");

await new Promise((resolve) => {
  const ws = new WebSocket(`${WS_URL}?token=${TOKEN}`);
  let connected = false;
  let gotHistory = false;
  let gotChatEcho = false;
  let chatContent = "";

  ws.on("open", () => {
    connected = true;
    ws.send(JSON.stringify({ type: "join_room", partyId: PARTY_ID }));

    setTimeout(() => {
      ws.send(JSON.stringify({ type: "chat", partyId: PARTY_ID, content: "E2E 测试消息" }));
    }, 500);
  });

  ws.on("message", (data) => {
    const lines = data.toString().split("\n");
    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        const msg = JSON.parse(line);
        if (msg.type === "chat_history") gotHistory = true;
        if (msg.type === "chat" && msg.content === "E2E 测试消息") {
          gotChatEcho = true;
          chatContent = msg.content;
        }
      } catch {}
    }
  });

  ws.on("error", (e) => console.log(`   ⚠️  ws error: ${e.message}`));

  setTimeout(() => {
    assert("连接成功", connected);
    assert("收到聊天历史", gotHistory);
    assert("聊天消息回显", gotChatEcho);
    ws.close();
    resolve();
  }, 3000);
});

// ---- 测试 3: 两个客户端交叉通信 ----
console.log("\n3️⃣  多人交叉通信测试");

await new Promise((resolve) => {
  const results = { u1: 0, u2: 0 };
  const end = () => {
    const bothGot = results.u1 >= 1 && results.u2 >= 1;
    assert("双方都能收到对方消息", bothGot);
    resolve();
  };

  let done = 0;
  function makeClient(label) {
    const ws = new WebSocket(`${WS_URL}?token=${TOKEN}`);
    ws.on("open", () => {
      ws.send(JSON.stringify({ type: "join_room", partyId: PARTY_ID }));
      setTimeout(() => {
        ws.send(JSON.stringify({ type: "chat", partyId: PARTY_ID, content: `${label} 消息` }));
      }, 800);
    });
    ws.on("message", (data) => {
      const lines = data.toString().split("\n");
      for (const line of lines) {
        if (!line.trim()) continue;
        try {
          const msg = JSON.parse(line);
          if (msg.type === "chat") results[label]++;
        } catch {}
      }
    });
    ws.on("error", () => {});
    return ws;
  }

  const ws1 = makeClient("U1");
  const ws2 = makeClient("U2");

  setTimeout(() => {
    end();
    ws1.close();
    ws2.close();
  }, 4000);
});

// ---- 结果 ----
console.log(`\n=== 📊 最终结果: ${passedTests}/${totalTests} 通过 ===`);
if (passedTests === totalTests) {
  console.log("✅ WebSocket 全链路正常！");
} else {
  console.log(`❌ 有 ${totalTests - passedTests} 个测试失败`);
  process.exit(1);
}
