# Splatoon 组队平台 — API 接口文档

## 架构

```text
Next.js 前端 ──HTTP──→ Go API (Gin) ──GORM──→ MySQL
                          │
                     JWT 中间件
                   (认证/权限校验)
```

原项目：Next.js Server Actions → Prisma → SQLite
新项目：Next.js Server Actions → fetch → Go API (Gin) → GORM → MySQL

> 前端代码已修改为调 REST API，不再直接操作数据库。

---

## 数据模型

### User（用户）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | String (UUID) | 主键 |
| email | String | 邮箱（唯一） |
| passwordHash | String | 密码哈希（PBKDF2） |
| displayName | String | 昵称 |
| role | Enum | USER / TRUSTED / ADMIN |
| gender | Enum | NOT_DISCLOSED / FEMALE / MALE / NON_BINARY / CUSTOM |
| genderNote | String? | 性别备注 |
| friendCode | String? | 好友码 |
| contact | String? | 联系方式 |
| bio | String? | 个人简介 |
| playStyle | String? | 游戏风格 |
| isBanned | Boolean | 是否封禁 |
| createdAt | DateTime | |
| updatedAt | DateTime | |

### Session（会话）

原项目用 Session + Cookie，Go 项目改用 JWT，此表不需要。

### Party（组队）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | String (UUID) | 主键 |
| game | String | 游戏名称 |
| mode | String | 游戏模式 |
| title | String | 标题 |
| startsAt | DateTime | 开始时间 |
| maxPlayers | Int | 最大人数（2-12） |
| voice | String | 语音方式 |
| region | String? | 区域 |
| note | String? | 备注 |
| contact | String | 联系方式 |
| genderMode | Enum | NO_PREFERENCE / WOMEN_FRIENDLY / SAME_GENDER / CUSTOM |
| genderPreferenceNote | String? | 性别偏好说明 |
| status | Enum | OPEN / FULL / CLOSED / HIDDEN |
| ownerId | String | 创建者 ID |
| createdAt | DateTime | |
| updatedAt | DateTime | |

### PartyMember（组队成员）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | String (UUID) | 主键 |
| partyId | String | 组队 ID |
| userId | String | 用户 ID |
| status | Enum | JOINED / LEFT / REMOVED |
| createdAt | DateTime | |
| updatedAt | DateTime | |

> 唯一约束：`[partyId, userId]`

### InviteCode（邀请码）

| 字段 | 类型 | 说明 |
|------|------|------|
| code | String | 邀请码（主键） |
| createdById | String? | 创建者 ID |
| redeemedById | String? | 使用者 ID |
| redeemedAt | DateTime? | 使用时间 |
| expiresAt | DateTime? | 过期时间 |
| createdAt | DateTime | |

### Report（举报）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | String (UUID) | 主键 |
| reason | String | 举报原因 |
| details | String? | 详情 |
| status | Enum | OPEN / REVIEWED / DISMISSED |
| partyId | String? | 被举报组队 ID |
| targetUserId | String? | 被举报用户 ID |
| reporterId | String? | 举报人 ID |
| createdAt | DateTime | |
| resolvedAt | DateTime? | 处理时间 |

---

## 接口清单

### 1️⃣ 用户认证

#### POST /api/register

注册（第一个注册用户自动成为 ADMIN，带邀请码注册成为 TRUSTED，否则为 USER）

```
Request:
{
  "email": "user@example.com",
  "password": "123456",
  "displayName": "玩家昵称",
  "inviteCode": "SQUID-START"    // 可选
}

Response 200:
{
  "token": "jwt_token_string",
  "user": {
    "id": "...",
    "email": "user@example.com",
    "displayName": "玩家昵称",
    "role": "TRUSTED"
  }
}

Error:
{ "message": "这个邮箱已经注册过" }
{ "message": "邀请码无效或已被使用" }
```

#### POST /api/login

```
Request:
{
  "email": "user@example.com",
  "password": "123456"
}

Response 200:
{
  "token": "jwt_token_string",
  "user": { ... }
}

Error:
{ "message": "邮箱或密码不正确" }
{ "message": "这个账号已被限制登录" }
```

#### POST /api/logout

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>

Response 200:
{ "message": "已退出" }
```

---

### 2️⃣ 用户资料

#### GET /api/profile

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>

Response 200:
{
  "id": "...",
  "email": "user@example.com",
  "displayName": "玩家昵称",
  "role": "TRUSTED",
  "gender": "NOT_DISCLOSED",
  "friendCode": "SW-xxxx-xxxx-xxxx",
  "contact": "联系方式",
  "bio": "个人简介",
  "playStyle": "游戏风格",
  "isBanned": false,
  "parties": [ ... ],   // 自己发布的组队列表
  "createdAt": "..."
}
```

#### PUT /api/profile

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>
Request:
{
  "displayName": "新昵称",
  "gender": "NOT_DISCLOSED",
  "genderNote": "",
  "friendCode": "SW-xxxx-xxxx-xxxx",
  "contact": "微信号",
  "playStyle": "打工、涂地",
  "bio": "休闲玩家"
}

Response 200:
{ "ok": true, "message": "资料已保存" }
```

---

### 3️⃣ 邀请码

#### POST /api/invite/redeem

需要 JWT 认证。USER 用户使用邀请码升级为 TRUSTED。

```
Headers: Authorization: Bearer <token>
Request:
{
  "code": "SQUID-START"
}

Response 200:
{ "ok": true, "message": "已升级为可信玩家，可以发布组队" }

Error:
{ "message": "邀请码无效或已被使用" }
```

#### POST /api/invite/create

需要 JWT 认证，仅 TRUSTED 及以上可用。

```
Headers: Authorization: Bearer <token>

Response 200:
{ "code": "ABCD1234" }

限制：每人每天最多创建 5 个（ADMIN 不限）
```

---

### 4️⃣ 组队 CRUD

#### POST /api/parties

需要 JWT 认证，仅 TRUSTED/ADMIN 可用。

```
Headers: Authorization: Bearer <token>
Request:
{
  "title": "今晚打工，轻松不压力",
  "game": "Splatoon 3",
  "mode": "鲑鱼跑",
  "startsAt": "2026-07-10T20:00:00Z",
  "maxPlayers": 4,
  "voice": "可不开麦",
  "region": "国区/港区均可",
  "contact": "加入后显示群号",
  "note": "欢迎新手",
  "genderMode": "NO_PREFERENCE",
  "genderPreferenceNote": ""
}

Response 200:
{
  "id": "party_id",
  ...
}

限制：每小时最多发布 3 个（ADMIN 不限）
```

#### GET /api/parties

公开接口。

```
Query:
  ?page=1
  &limit=20
  &game=Splatoon 3      // 可选筛选
  &mode=鲑鱼跑            // 可选筛选
  &status=OPEN           // 可选筛选

Response 200:
{
  "parties": [ ... ],
  "total": 50,
  "page": 1,
  "limit": 20
}
```

#### GET /api/parties/:id

公开接口。

```
Response 200:
{
  "id": "...",
  "title": "...",
  "game": "...",
  "mode": "...",
  "startsAt": "...",
  "maxPlayers": 4,
  "voice": "...",
  "region": "...",
  "contact": "...",         // 对已登录用户可见
  "note": "...",
  "genderMode": "...",
  "status": "OPEN",
  "owner": {                // 创建者信息
    "id": "...",
    "displayName": "...",
    "friendCode": "...",
    "playStyle": "..."
  },
  "ownerId": "...",
  "members": [ ... ],       // 已加入成员
  "currentPlayers": 2,      // 当前人数
  "createdAt": "..."
}
```

#### DELETE /api/parties/:id

需要 JWT 认证，仅创建者或 ADMIN。

```
Headers: Authorization: Bearer <token>

Response 200:
{ "message": "已关闭" }
```

#### POST /api/parties/:id/join

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>

Response 200:
{ "message": "已加入" }

限制：每小时最多加入 10 次
满人数自动变 FULL 状态
```

#### POST /api/parties/:id/leave

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>

Response 200:
{ "message": "已退出" }

如果原来是 FULL，退出后自动变 OPEN
```

---

### 5️⃣ 举报

#### POST /api/parties/:id/report

需要 JWT 认证。

```
Headers: Authorization: Bearer <token>
Request:
{
  "reason": "广告/骚扰",
  "details": "详细说明"
}

Response 200:
{ "ok": true, "message": "已收到举报，管理员会处理" }

限制：每天最多举报 10 次
同一组队累计 3 次举报自动隐藏（status = HIDDEN）
```

---

### 6️⃣ 管理员

#### GET /api/admin/reports

需要 JWT 认证，仅 ADMIN。

```
Response 200:
{
  "reports": [
    {
      "id": "...",
      "reason": "广告",
      "details": "...",
      "status": "OPEN",
      "party": { ... },
      "reporter": { ... },
      "createdAt": "..."
    }
  ]
}
```

#### POST /api/admin/reports/:id/resolve

需要 JWT 认证，仅 ADMIN。

```
Headers: Authorization: Bearer <token>
Request:
{
  "action": "dismiss"    // dismiss=忽略, hide=隐藏组队, ban=封禁
}

Response 200:
{ "message": "已处理" }
```

---

### 7️⃣ 健康检查

#### GET /api/health

```
Response 200:
{
  "status": "ok",
  "db": "connected"
}
```

---

## 权限矩阵

| 功能 | 未登录 | USER | TRUSTED | ADMIN |
|------|--------|------|---------|-------|
| 浏览组队列表 | ✅ | ✅ | ✅ | ✅ |
| 查看组队详情 | ✅ | ✅ | ✅ | ✅ |
| 注册/登录 | ✅ | - | - | - |
| 修改个人资料 | ❌ | ✅ | ✅ | ✅ |
| 加入组队 | ❌ | ✅ | ✅ | ✅ |
| 举报 | ❌ | ✅ | ✅ | ✅ |
| 发布组队 | ❌ | ❌ | ✅ | ✅ |
| 创建邀请码 | ❌ | ❌ | ✅ | ✅ |
| 使用邀请码升级 | ❌ | ✅ | - | - |
| 关闭/删除组队 | ❌ | ❌ | 仅自己的 | 全部 |
| 处理举报 | ❌ | ❌ | ❌ | ✅ |
| 封禁用户 | ❌ | ❌ | ❌ | ✅ |
| 查看管理员面板 | ❌ | ❌ | ❌ | ✅ |

---

## 频率限制

| 操作 | 限制 |
|------|------|
| 发布组队 | 每小时最多 3 个（ADMIN 不限） |
| 加入组队 | 每小时最多 10 次 |
| 创建邀请码 | 每天最多 5 个（ADMIN 不限） |
| 举报 | 每天最多 10 次 |
| 自动隐藏 | 同一组队累计 3 次举报 |

---

## 原项目使用的技术

| 组件 | 原项目 | Go 项目替换 |
|------|--------|-----------|
| 框架 | Next.js Server Actions | Gin |
| 数据库 | SQLite (Prisma) | MySQL (GORM) |
| 认证 | Session + Cookie | JWT |
| 密码 | PBKDF2 | bcrypt 或相同 |
| 校验 | Zod | go-playground/validator |
| 部署 | 未部署 | Docker |
