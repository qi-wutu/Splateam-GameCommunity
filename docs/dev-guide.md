# Splatoon 项目 — 开发指南

## 项目结构

```text
D:\CCWorkSpace\
├── splatoon-backend/       # Go 后端（你在写的）
│   ├── main.go
│   ├── config/
│   │   ├── .env             # 配置文件（不提交 Git）
│   │   └── docker-compose.yml
│   ├── models/
│   ├── handlers/
│   ├── middleware/
│   └── docs/
└── splatoon-web-app-ai/    # Next.js 前端（已有的）
    ├── src/
    │   ├── app/            # 页面 + Server Actions
    │   ├── components/     # UI 组件
    │   └── lib/            # API 客户端、认证工具
    ├── package.json
    └── .env                # NEXT_PUBLIC_API_URL
```

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
docker-compose up -d --build
```

## API 基地址

- 开发环境：`http://localhost:8080/api`
- 前端通过 `.env.local` 的 `NEXT_PUBLIC_API_URL` 配置

## 关键变动记录

### 2026-07-03：架构从 Prisma 直连改为 REST API

- 原架构：Next.js Server Actions → Prisma → SQLite
- 新架构：Next.js Server Actions → fetch → Go API (Gin) → GORM → MySQL
- 认证从 Session/Cookie 改为 JWT
- 前端的 Prisma 依赖已移除，所有数据通过 HTTP 调 Go 后端
