#!/bin/bash
# WebSocket E2E 测试运行器（Windows 兼容）
# DIR = 脚本所在目录（ws_test/）
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "d:/CCWorkSpace/splatoon-backend"

echo "=== 🚀 E2E 测试开始 ==="

# 1. 清理旧进程
taskkill //F //IM splatoon-test.exe 2>/dev/null

# 2. 重新编译
go build -o "$DIR/splatoon-test.exe" . 2>&1 || { echo "❌ 编译失败"; exit 1; }

# 3. 启动后端，输出到文件
"$DIR/splatoon-test.exe" > backend-e2e.log 2>&1 &
PID=$!
sleep 3

curl -s http://localhost:8080/api/party > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "❌ 后端启动失败"
  cat backend-e2e.log
  exit 1
fi
echo "✅ 后端已启动"

# 4. 注册
curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"e2euser@test.com","userName":"E2EUser","password":"123456","gender":"male"}' > /dev/null
sleep 1

# 5. 取激活码
CODE=$(grep -o "激活码 \[e2euser@test.com\]: [a-f0-9]*" backend-e2e.log | grep -o "[a-f0-9]\{6\}")
echo "激活码: $CODE"

# 6. 激活
curl -s -X POST http://localhost:8080/api/auth/activate \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"e2euser@test.com\",\"code\":\"$CODE\"}" > /dev/null

# 7. 登录
LOGIN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"e2euser@test.com","password":"123456"}')
TOKEN=$(echo "$LOGIN" | sed 's/.*"token":"\([^"]*\)".*/\1/')
echo "Token 长度: ${#TOKEN}"

# 8. 创建组队
PARTY=$(curl -s -X POST http://localhost:8080/api/party \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title":"E2E","game":"Splatoon 3","introduction":"x","playernum":1,"maxNum":4}')
PARTY_ID=$(echo "$PARTY" | sed 's/.*"ID":\([0-9]*\).*/\1/')
echo "Party ID: $PARTY_ID"

# 9. 把 token 和 party_id 写入文件，node 读取
echo -n "$TOKEN" > "$DIR/e2e_token.txt"
echo -n "$PARTY_ID" > "$DIR/e2e_party.txt"
echo "变量已写入 e2e_*.txt"

# 10. 运行 Node.js 测试
echo ""
echo "========================================"
node "$DIR/e2e-test.mjs" 2>&1
echo "========================================"

# 11. 清理
taskkill //F //IM splatoon-test.exe 2>/dev/null
rm -f backend-e2e.log "$DIR/e2e_token.txt" "$DIR/e2e_party.txt"
echo ""
echo "=== 🏁 测试完成 ==="
