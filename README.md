# Splatoon 实时组队平台 · 后端

一个用 Go 实现的 **Splatoon 游戏组队平台后端** —— 不止于 CRUD，还带 **WebSocket 实时聊天**、**RabbitMQ 异步邮件**、**Redis 在线状态缓存**与**优雅降级**。作为一个能讲清"为什么这么设计"的完整后端。

> 配套：`splatoon-front` / `splatoon_fronted` / `splatoon-web-app-ai` / `splatoon_platform` 为工作区里的兄弟前端项目（本仓库仅后端）。

---

## 技术栈

| 层 | 技术 |
|---|---|
| 语言 / 框架 | Go 1.26 · Gin |
| 数据库 | MySQL 8.0 + GORM（AutoMigrate，自动建表） |
| 认证 | JWT（HS256，72h）+ bcrypt 密码哈希 |
| 实时通讯 | gorilla/websocket |
| 消息队列 | RabbitMQ（amqp091-go，topic 交换机） |
| 缓存 / 状态 | Redis（go-redis/v9） |
| 邮件 | net/smtp（QQ 邮箱 / 通用 SMTP） |

---

## 亮点特性

**账号体系**
- 注册 → bcrypt 哈希 → 生成 6 位激活码（Redis，30 分钟 TTL，Redis 不可用时内存兜底）→ **RabbitMQ 异步**发送激活邮件
- 登录 → 签发 JWT（HS256，携带 `user_id` + `exp`）；`bcrypt` 比对密码
- 激活校验 + JWT 启动自检（`ValidateJWT`：**空 / 公开默认值 / 过短密钥直接拒绝启动**，杜绝认证绕过）

**组队（Party）**
- 创建 / 列表 / 详情（含成员）/ 删除（仅创建者，软删成员）
- 加入 / 退出：**JoinParty 已做人数上限校验**（`playernum < MaxNum`），人满拒绝；退队硬删避免唯一索引冲突

**实时聊天（WebSocket）**
- 全局 `Hub`（单 RWMutex 管理 `Clients` / `Rooms`），每连接独立 `ReadPump` / `WritePump` 双 goroutine
- 心跳 ping/pong，写缓冲(256) 满时**非阻塞丢包**（有意的背压策略）
- 聊天消息经 Redis 缓存**最近 100 条**，进房拉历史；在线状态写 Redis 带 TTL
- 同一 `user_id` 重复连接会**踢掉旧连接**（防重复登录）

**异步邮件链路**
- 注册 → 激活码（Redis/内存）→ MQ(topic `splatoon.events`) → `mail.welcome` 队列 → `mail_worker` 消费 → SMTP 发信（手动 ack）

**优雅降级（生产可用思维）**
- Redis / RabbitMQ / SMTP **连接失败均非致命**：打印警告并降级（激活码打印到控制台 / 消息走内存 / 邮件跳过），不影响启动

**性能压测工具**（`ws_test/`）
- k6（两种模式：并发连接数 / 消息吞吐）、端到端 e2e 脚本、独立的 Go 负载生成器
- 实测：单机渐进建连可支撑约 **3 万条并发长连接（握手零失败）**；同房间 200 客户端广播约 24.7 万消息/秒（约 38% 因缓冲满被有意丢弃）
- 详见 [ws_test/README-k6.md](ws_test/README-k6.md)

---

## 架构

```
客户端（浏览器/前端）
   │  ── HTTP REST ────────────────────────── WebSocket ──  │
   ▼                                                        ▼
┌──────────────────────────────────────────────────────────────────┐
│  Gin (router.go) + middlewares: CORS · AuthMiddleware(JWT)       │
│  ├─ controller_user.go  ─► service/auth_service.go               │
│  ├─ controller_party.go ─► service/party_service.go              │
│  └─ controller_ws.go    ─► service/ws_service.go  (Hub)          │
└──────────────┬───────────────────────────────┬───────────────────┘
               │ GORM                          │ Redis / MQ / SMTP
               ▼                               ▼
        MySQL                                  Redis     ──► RabbitMQ ──► mail_worker ──► SMTP
   User / Party / PartyMember          在线状态·消息缓存·激活码
```

**模块职责**
- `controller/` — 参数解析 / 绑定 JSON / 返回响应
- `service/` — 业务逻辑；`ws_service.go` 为 WebSocket Hub（在线表、房间、广播）
- `middlewares/` — CORS 与 JWT 认证中间件
- `models/` — GORM 数据模型
- `router/` — 路由注册
- `config/` — 环境变量配置 + MySQL/Redis/RabbitMQ 初始化

---

## 库表关系

```
User  (id uint PK, email unique, password, user_name, gender, active)
   │ 1
   │
   ├──── N  Party      Party.owner_id → User.id（创建者）
   │
   └──── N  PartyMember   PartyMember.user_id → User.id
                             PartyMember.party_id → Party.id

Party      (id uint PK, title, game, introduction, playernum, max_num, owner_id, owner_name)
PartyMember(id uint PK, party_id, user_id, status)
```

---

## 目录结构

```
├── main.go          # 入口：LoadEnv → ValidateJWT → InitMySQL/Redis/RabbitMQ → 启动 Hub + mail worker
├── config/          # .env / .env.example / config.go / redis.go / rabbitmq.go
├── controller/      # HTTP 处理
├── service/         # auth / party / ws_service(Hub) / mail_worker
├── middlewares/     # CORS、AuthMiddleware
├── models/          # User / Party / PartyMember
├── router/          # 路由
├── utils/           # JWT 签发/解析、bcrypt
├── docs/            # roadmap.md（公开文档）
└── ws_test/         # 压测、e2e
```

---

## 本地部署

前端先配置好环境变量（参照 `.env.example`）：

```bash
cp config/.env.example config/.env
# 编辑 config/.env：DB_*、JWT_SECRET（≥32字符强随机）、SMTP_*（可选）
```

所需依赖：

| 依赖 | 是否必须 | 不配会怎样 |
|---|---|---|
| MySQL | **必须** | 启动 panic |
| Redis | 可选 | 在线状态/消息缓存/激活码降级（内存兜底） |
| RabbitMQ | 可选 | 激活码打印到控制台，邮件 worker 不启动 |
| JWT_SECRET | **必须**（强随机≥32） | `ValidateJWT` 拒绝启动 |

启动：

```bash
go run .
# 可用：http://localhost:8080
```

> 注：数据库表由 GORM `AutoMigrate` 自动创建；`SERVER_PORT` 默认 `8080`。

---

## API 接口

### 公开

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/register` | 注册（返回用户信息 + 触发激活邮件） |
| POST | `/api/auth/login` | 登录（返回 JWT token） |
| POST | `/api/auth/activate` | 邮箱激活（body: `{email, code}`） |
| GET | `/api/party?limit=&cursor=` | 组队列表（**游标分页**，返回 `{items, nextCursor, hasMore}`） |
| GET | `/api/party/:id` | 组队详情（含成员） |
| GET | `/api/ws?token=` | WebSocket 握手（`token` 为 JWT） |

> `GET /api/party` 走**游标分页 + Redis ZSET 索引缓存**：`limit`（默认 10，上限 50）+ `cursor`（来自上一页 `nextCursor`，首屏省略）。响应为 `{items, nextCursor, hasMore}`。入口见 [service/party_cache.go](service/party_cache.go)。

### 需认证（`Authorization: Bearer <token>`）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/user/me` | 当前用户信息 |
| GET | `/api/user/:id/online` | 用户是否在线 |
| POST | `/api/party` | 创建组队（创建者自动入队，`playernum=1`） |
| DELETE | `/api/party/:id` | 删除组队（仅创建者） |
| POST | `/api/party/:id/join` | 加入组队（**满员拒绝**） |
| POST | `/api/party/:id/leave` | 退出组队 |

---

## 相关文档

- [docs/roadmap.md](docs/roadmap.md) — Roadmap / 待办（含全局 Hub 单锁并发瓶颈 issue）
- [ws_test/README-k6.md](ws_test/README-k6.md) — WebSocket 压测说明与实测结果

## License

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)