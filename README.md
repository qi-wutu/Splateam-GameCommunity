# Splatoon 组队平台 (后端)

一个用 Go 实现的 Splatoon 游戏组队平台后端 API。

## 技术栈

- **语言：** Go 1.26
- **框架：** Gin
- **数据库：** MySQL + GORM
- **认证：** JWT (bcrypt 密码哈希)

## 项目结构

```
├── main.go              # 程序入口
├── config/
│   ├── .env             # 环境变量
│   └── config.go        # 配置 & 数据库初始化
├── controller/          # HTTP 请求处理
├── service/             # 业务逻辑层
├── models/              # 数据模型
├── middlewares/         # Gin 中间件 (JWT 认证)
├── utils/               # 工具函数 (JWT 生成/解析)
├── router/              # 路由注册
├── docs/                # 公开文档（进仓库，如 roadmap.md）
└── docs_local/          # 本地私有文档（被 .gitignore 忽略，不进仓库）
```

## 启动方式

### 前置要求

- Go 1.21+
- MySQL 8.0+

### 1. 配置数据库

在 `config/.env` 中填写数据库信息（已默认配置）：

```
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=splatoon
```

### 2. 创建数据库

```sql
CREATE DATABASE splatoon CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 启动

```bash
go run main.go
```

服务启动在 `http://localhost:8080`，数据库表会自动迁移创建。

## API 接口

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/register` | 注册 |
| POST | `/api/auth/login` | 登录 |
| GET | `/api/party` | 组队列表 |
| GET | `/api/party/:id` | 组队详情 |

### 需要认证的接口

在请求头中带上 `Authorization: Bearer <token>`：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/party` | 创建组队 |
| DELETE | `/api/party/:id` | 删除组队（仅创建者） |
| POST | `/api/party/:id/join` | 加入组队 |
| POST | `/api/party/:id/leave` | 退出组队 |

## 测试流程

```bash
# 1. 注册
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"123456","userName":"Tester"}'

# 2. 登录（保存返回的 token）
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"123456"}'

# 3. 创建组队（替换 <token>）
curl -X POST http://localhost:8080/api/party \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"今晚打工","game":"Splatoon 3","maxNum":4}'

# 4. 查看列表
curl http://localhost:8080/api/party
```

## 开发状态

MVP 已完成，核心功能：
- ✅ 用户注册 / 登录
- ✅ JWT 认证中间件
- ✅ 组队 CRUD（创建 / 列表 / 详情 / 删除）
- ✅ 加入 / 退出组队
- ✅ 软删除
