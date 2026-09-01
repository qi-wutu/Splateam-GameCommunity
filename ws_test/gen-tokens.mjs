// 生成 N 个独立 user_id 的合法 JWT，输出到 tokens.txt（每行一个）。
//
// 为什么需要独立 user_id：服务端 Hub 会按 user_id 踢掉旧的同 ID 连接
// （service/ws_service.go 的 Register 分支：close(old.Send); old.Conn.Close()）。
// 若所有 VU 共用同一个 token，第二个连接会把第一个挤下线，测出来只是"1 个连接"。
//
// 用法：node gen-tokens.mjs <数量> [JWT_SECRET] [前缀]
//   node gen-tokens.mjs 5000 splatoon-dev-secret-key load
//
// 生成的 token 用 HS256 + 与服务器一致的 secret 签名，claim 只含 user_id + exp，
// 与 utils/utils.go 的 GenerateJWT 一致，会在 ParseJWT（仅校验签名 + user_id）时通过。
// 这些 user_id 不需要真实存在于数据库：chat 时 GetUserByID 查不到会 fallback 用 userID 当昵称。
import { createHmac } from "node:crypto";
import { writeFileSync } from "node:fs";

const N = parseInt(process.argv[2] || "1000", 10);
// 优先取第 3 个参数（允许为空字符串）；否则取环境变量；再否则用默认值。
// 注意：若服务端 utils.InitJWT() 未被调用（jwtSecret 为空），生成 token 需传空 secret 才能被接受。
const SECRET = process.argv.length > 3 ? process.argv[3] : (process.env.JWT_SECRET || "splatoon-dev-secret-key");
const PREFIX = process.argv[4] || "load";

// base64url（JWT 要求无 padding）
const b64url = (s) => Buffer.from(s).toString("base64url");
const header = b64url(JSON.stringify({ alg: "HS256", typ: "JWT" }));
const exp = Math.floor(Date.now() / 1000) + 72 * 3600; // 与 utils 一致：72 小时

const lines = new Array(N);
for (let i = 0; i < N; i++) {
  const userId = `${PREFIX}-${i}`;
  const payload = b64url(JSON.stringify({ user_id: userId, exp }));
  const signingInput = `${header}.${payload}`;
  const sig = createHmac("sha256", SECRET).update(signingInput).digest("base64url");
  lines[i] = `${signingInput}.${sig}`;
}

const out = new URL("./tokens.txt", import.meta.url); // 传 URL 对象，Node 跨平台处理路径
writeFileSync(out, lines.join("\n") + "\n");
console.log(`✔ 已生成 ${N} 个 JWT → ${out.pathname}`);
