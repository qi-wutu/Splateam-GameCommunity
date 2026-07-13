---
name: cors-config
description: CORS 跨域问题说明及本项目的配置方式
metadata:
  type: project
  tags: [cors, middleware, frontend-backend]
---

## CORS 是什么

CORS（Cross-Origin Resource Sharing，跨域资源共享）是一种浏览器安全机制。

### 同源策略

浏览器默认只允许网页请求**同源**的资源。"同源"指协议、域名、端口三者完全一致。

前端页面 `http://localhost:3000` 请求后端 `http://localhost:8080` 的 API 时：
- ✅ 协议相同（http）
- ✅ 域名相同（localhost）
- ❌ **端口不同（3000 ≠ 8080）**

浏览器视为跨域，默认拦截请求。这就是前端报 `fail to fetch` 的原因—请求实际上发出了，后端也正常返回了，但浏览器检查到跨域后把响应丢掉了。

### CORS 的工作原理

后端在 HTTP 响应头中声明允许跨域访问，浏览器根据这些头决定是否放行：

```
Access-Control-Allow-Origin: http://localhost:3000
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

对于非简单请求（如带 `Authorization` 头的请求），浏览器会先发一个 `OPTIONS` 预检请求，检查服务端是否允许。预检通过后才发实际请求。

## 本项目的 CORS 配置

文件：`middlewares/middlewares.go`

```go
func CORS() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        ctx.Header("Access-Control-Allow-Origin", "*")
        ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if ctx.Request.Method == "OPTIONS" {
            ctx.AbortWithStatus(204)
            return
        }
        ctx.Next()
    }
}
```

注册到全局路由（`router/router.go`）：

```go
r := gin.Default()
r.Use(middlewares.CORS())
```

### 配置说明

| 响应头 | 值 | 说明 |
|--------|----|------|
| `Access-Control-Allow-Origin` | `*` | 允许所有来源（上线前应改为具体域名） |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, DELETE, OPTIONS` | 允许的 HTTP 方法 |
| `Access-Control-Allow-Headers` | `Content-Type, Authorization` | 允许的自定义请求头（Authorization 用于 JWT） |

## 为什么之前没配也能跑 curl

curl 不受同源策略限制。所以命令行测试一直正常，但浏览器页面调用就报错。

```
curl -X POST http://localhost:8080/api/auth/login  ← 正常
浏览器 JS fetch() → 同源策略拦截 → fail to fetch  ← 报错
```

## 上线前要注意

`Access-Control-Allow-Origin: *` 意味着任何网站都可以调用你的 API。上线前应改为具体的域名：

```go
ctx.Header("Access-Control-Allow-Origin", "https://your-frontend-domain.com")
```

## 相关文件

- `middlewares/middlewares.go` — CORS 和 Auth 中间件
- `router/router.go` — 中间件注册
