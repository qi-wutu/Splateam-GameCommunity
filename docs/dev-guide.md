# Splatoon 项目 — 开发指南

## 项目结构

```text
D:\CCWorkSpace\
├── splatoon-backend/         # Go 后端（你在写的）
│   ├── main.go               # 入口：加载配置 → 连接数据库 → 启动路由
│   ├── config/
│   │   ├── config.go         # 读取 .env、初始化 MySQL、全局 Db/ServerPort
│   │   ├── .env              # 配置文件（不提交 Git）
│   │   └── docker-compose.yml
│   ├── models/               # 数据模型（GORM）
│   │   └── models.go
│   ├── repository/           # 数据库操作层（GORM CRUD）
│   │   └── repository.go
│   ├── handlers/             # HTTP 处理层（TODO：收请求、返回 JSON）
│   ├── middleware/           # 中间件（TODO：JWT 鉴权）
│   ├── router/               # 路由注册
│   │   └── router.go
│   └── docs/
│       ├── api-reference.md  # API 接口文档
│       └── dev-guide.md      # 本文件
└── splatoon-web-app-ai/      # Next.js 前端（已有的）
    ├── src/
    │   ├── app/              # 页面 + Server Actions
    │   ├── components/       # UI 组件
    │   └── lib/              # API 客户端、认证工具
    ├── package.json
    └── .env                  # NEXT_PUBLIC_API_URL
```

## 架构分层

```text
main.go
  ↓
config.LoadEnv() → 读取 .env
config.InitMySQL() → 连接 MySQL，赋值全局 config.Db
  ↓
router.SetupRouter() → 注册路由，返回 *gin.Engine
  ↓
r.Run() → 启动 HTTP 服务

路由处理流程：
handler（收请求、校验参数）
  → service（业务逻辑）             ← TODO
    → repository（GORM 数据库操作）  ← TODO
```

## 关键代码文件

| 文件 | 职责 |
|------|------|
| `main.go` | 仅编排启动流程，不写业务代码 |
| `config/config.go` | 定义 `LoadEnv()`、`InitMySQL()`、全局变量 `Db` 和 `ServerPort` |
| `router/router.go` | 定义 `SetupRouter()`，注册所有路由 |
| `models/models.go` | GORM 数据模型定义 |
| `repository/repository.go` | GORM 操作封装（当前为空） |

## 启动方式

### 后端（Go）

```bash
cd D:\CCWorkSpace\splatoon-backend
go run main.go
# 监听 :8080
```

### 前端（Next.js）

```bash
cd D:\CCWorkSpace\splatoon-web-app-ai
npm install
npm run dev
# 监听 :3000
```

### 生产部署

```bash
cd D:\CCWorkSpace\splatoon-backend
docker compose -f config/docker-compose.yml up -d
```

## API 基地址

- 开发环境：`http://localhost:8080/api`
- 前端通过 `.env` 的 `NEXT_PUBLIC_API_URL` 配置

## 关键变动记录

### 2026-07-03：架构从 Prisma 直连改为 REST API

- 原架构：Next.js Server Actions → Prisma → SQLite
- 新架构：Next.js Server Actions → fetch → Go API (Gin) → GORM → MySQL
- 认证从 Session/Cookie 改为 JWT
- 前端的 Prisma 依赖已移除，所有数据通过 HTTP 调 Go 后端

### 2026-07-03：后端重构为三层架构

- main.go 拆分为 config（配置+数据库）、router（路由）、models、repository
- 采用三层架构：handler → service → repository（逐步建设中）
- 配置文件统一移到 config/ 目录下
