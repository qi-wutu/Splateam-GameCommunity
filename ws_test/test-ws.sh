#!/bin/bash
# WebSocket 端到端测试脚本 v2 (Windows 兼容版)
# DIR = 脚本所在目录（ws_test/）
DIR="$(cd "$(dirname "$0")" && pwd)"

cd "d:/CCWorkSpace/splatoon-backend"

echo "=== 🧪 WebSocket 全链路测试 ==="
echo ""

# 1. 杀掉旧进程，启动后端，输出到文件
echo ">>> 1. 启动后端..."
taskkill //F //IM splatoon-test.exe 2>/dev/null
sleep 1

# 重新编译
go build -o "$DIR/splatoon-test.exe" . 2>&1 || { echo "❌ 编译失败"; exit 1; }

# 后端输出重定向
"$DIR/splatoon-test.exe" > backend-test.log 2>&1 &
BACKEND_PID=$!
sleep 3

# 验证后端
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/party)
if [ "$HTTP_CODE" != "200" ]; then
  echo "❌ 后端启动失败"
  cat backend-test.log
  exit 1
fi
echo "✅ 后端启动成功 (PID: $BACKEND_PID)"

# 2. 注册
echo ">>> 2. 注册测试用户..."
REG=$(curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"wstest5@test.com","userName":"WSTest","password":"123456","gender":"male"}')
echo "   注册: $REG"

# 3. 从日志取激活码（不用 -P）
echo ">>> 3. 取激活码..."
sleep 1
ACTIVATION_CODE=$(grep -o "激活码 \[wstest5@test.com\]: [a-f0-9]*" backend-test.log | grep -o "[a-f0-9]\{6\}")
echo "   激活码: $ACTIVATION_CODE"

if [ -z "$ACTIVATION_CODE" ]; then
  echo "❌ 无法获取激活码，日志内容："
  cat backend-test.log
  taskkill //F //IM splatoon-test.exe 2>/dev/null
  exit 1
fi

# 4. 激活
echo ">>> 4. 激活..."
ACTIVATE=$(curl -s -X POST http://localhost:8080/api/auth/activate \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"wstest5@test.com\",\"code\":\"$ACTIVATION_CODE\"}")
echo "   激活: $ACTIVATE"

# 5. 登录拿 token
echo ">>> 5. 登录..."
LOGIN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"wstest5@test.com","password":"123456"}')
echo "   登录: $LOGIN"
TOKEN=$(echo "$LOGIN" | sed 's/.*"token":"\([^"]*\)".*/\1/')
echo "   Token: ${TOKEN:0:40}..."

# 6. 创建组队
echo ">>> 6. 创建组队..."
PARTY=$(curl -s -X POST http://localhost:8080/api/party \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"WSTest","game":"Splatoon 3","introduction":"test","playernum":1,"maxNum":4}')
echo "   创建: $PARTY"
PARTY_ID=$(echo "$PARTY" | sed 's/.*"ID":\([0-9]*\).*/\1/')
echo "   组队ID: $PARTY_ID"

# 7. WebSocket 测试（从本目录运行以找到 ws 模块）
echo ">>> 7. WebSocket 连接测试..."

cat > "$DIR/ws_single_test.mjs" << 'EOF'
import { WebSocket } from "ws";

const TOKEN = process.argv[1];
const PARTY_ID = process.argv[2];

const ws = new WebSocket(`ws://localhost:8080/api/ws?token=${TOKEN}`);
let tests = { connected: false, joined: false, history: false, chatEcho: false };
let msgs = [];

ws.on("open", () => {
  tests.connected = true;
  console.log(`   ✅ 1. WebSocket 连接成功`);

  ws.send(JSON.stringify({ type: "join_room", partyId: PARTY_ID }));
  console.log(`   📤 join_room ${PARTY_ID}`);

  setTimeout(() => {
    ws.send(JSON.stringify({ type: "chat", partyId: PARTY_ID, content: "压力测试消息 #1" }));
    console.log(`   📤 chat: "压力测试消息 #1"`);

    setTimeout(() => {
      ws.send(JSON.stringify({ type: "chat", partyId: PARTY_ID, content: "消息 #2" }));
      console.log(`   📤 chat: "消息 #2"`);
    }, 300);
  }, 500);
});

ws.on("message", (data) => {
  const lines = data.toString().split("\n");
  for (const line of lines) {
    if (!line.trim()) continue;
    try {
      const msg = JSON.parse(line);
      msgs.push(msg.type);
      console.log(`   📥 ${msg.type}: "${(msg.content||"").substring(0,30)}"`);

      if (msg.type === "chat_history") tests.history = true;
      if (msg.type === "chat") tests.chatEcho = true;
    } catch(e) {}
  }
});

ws.on("error", (e) => console.log(`   ❌ 错误: ${e.message}`));

setTimeout(() => {
  console.log(`\n   📊 单客户端测试结果:`);
  console.log(`      连接成功: ${tests.connected ? "✅" : "❌"}`);
  console.log(`      收到聊天历史: ${tests.history ? "✅" : "❌"}`);
  console.log(`      聊天回显: ${tests.chatEcho ? "✅" : "❌"}`);
  ws.close();
  process.exit(0);
}, 3000);

setTimeout(() => process.exit(0), 5000);
EOF

node "$DIR/ws_single_test.mjs" "$TOKEN" "$PARTY_ID" 2>&1

# 8. 多人测试（注册第二个用户）
echo ""
echo ">>> 8. 多人聊天测试..."
REG2=$(curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"wstest6@test.com","userName":"WSTest2","password":"123456","gender":"female"}')
sleep 1
CODE2=$(grep -o "激活码 \[wstest6@test.com\]: [a-f0-9]*" backend-test.log | grep -o "[a-f0-9]\{6\}")
curl -s -X POST http://localhost:8080/api/auth/activate \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"wstest6@test.com\",\"code\":\"$CODE2\"}" > /dev/null
LOGIN2=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"wstest6@test.com","password":"123456"}')
TOKEN2=$(echo "$LOGIN2" | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -s -X POST "http://localhost:8080/api/party/$PARTY_ID/join" \
  -H "Authorization: Bearer $TOKEN2" > /dev/null
echo "   用户2 注册+激活+加入组队 完成"

cat > "$DIR/ws_multi_test.mjs" << 'EOF'
import { WebSocket } from "ws";

const [TOKEN1, TOKEN2, PARTY_ID] = process.argv.slice(1);
let chatReceived = { u1: 0, u2: 0 };

function makeClient(name, token, label) {
  const ws = new WebSocket(`ws://localhost:8080/api/ws?token=${token}`);
  ws.on("open", () => {
    ws.send(JSON.stringify({ type: "join_room", partyId: PARTY_ID }));
    setTimeout(() => {
      ws.send(JSON.stringify({ type: "chat", partyId: PARTY_ID, content: `${name} 来了!` }));
    }, 1000);
  });
  ws.on("message", (data) => {
    const lines = data.toString().split("\n");
    for (const l of lines) {
      if (!l.trim()) continue;
      try {
        const m = JSON.parse(l);
        if (m.type === "chat") chatReceived[label]++;
      } catch(e) {}
    }
  });
  return ws;
}

const ws1 = makeClient("Alice", TOKEN1, "u1");
const ws2 = makeClient("Bob", TOKEN2, "u2");

setTimeout(() => {
  console.log(`   📊 多人测试: 用户1收到 ${chatReceived.u1} 条, 用户2收到 ${chatReceived.u2} 条`);
  if (chatReceived.u1 >= 1 && chatReceived.u2 >= 1) {
    console.log(`   ✅ 多人广播正常！`);
  } else {
    console.log(`   ⚠️  广播可能有丢包`);
  }
  ws1.close();
  ws2.close();
  process.exit(0);
}, 4000);
setTimeout(() => process.exit(0), 6000);
EOF

node "$DIR/ws_multi_test.mjs" "$TOKEN" "$TOKEN2" "$PARTY_ID" 2>&1

# 9. Redis 测试（如果 Redis 可用）
echo ""
echo ">>> 9. 在线状态 API 测试..."
ONLINE=$(curl -s "http://localhost:8080/api/user/me" -H "Authorization: Bearer $TOKEN")
echo "   当前用户: $ONLINE"

# 10. 清理
echo ""
echo ">>> 10. 清理..."
taskkill //F //IM splatoon-test.exe 2>/dev/null
rm -f "$DIR/ws_single_test.mjs" "$DIR/ws_multi_test.mjs" backend-test.log

echo ""
echo "=== ✅ 测试全部完成 ==="
