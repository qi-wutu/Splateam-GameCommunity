---
name: parsejwt-duplicate-bearer
description: "ParseJWT failed 问题排查：Postman 发现后端 JWT 多余 Bearer 前缀"
metadata:
  type: project
  tags: [jwt, auth, frontend-backend, debug]
---

## 问题现象

前端创建/加入组队时，控制台报 `ParseJWT failed`，所有需要鉴权的接口均无法使用。

## 排查过程

### 1. 定位错误来源

后端 `middlewares/middlewares.go:33` 在 `utils.ParseJWT(token)` 返回 error 时输出：

```go
ctx.JSON(400, gin.H{"error": "ParseJWT failed"})
```

说明 token 传到后端解析时已经不是一个合法的 JWT 了。

### 2. 通过 Postman 发现问题

用 Postman 直接请求后端登录接口，观察返回的 token：

```
{
  "token": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": { ... }
}
```

发现后端返回的 **token 字段本身已经带了 `Bearer ` 前缀**。

再看前端的 `authRequest()` 逻辑（`lib/api.ts`）：

```typescript
Authorization: `Bearer ${token}`,
```

前端发请求时也会在 token 前面加一个 `Bearer `。

### 3. 根因

后端返回的 token 已经是 `"Bearer eyJ..."`，前端存到 cookie 后发请求时又拼一次 `Bearer `，最终请求头变成：

```
Authorization: Bearer Bearer eyJhbGciOiJ...
```

**双重 `Bearer ` 前缀**。后端 `ParseJWT()` 去掉一个 `"Bearer "` 后还剩 `"Bearer eyJ..."`，不是合法 JWT 格式，解析失败。

### 4. 解决

通过 Postman 确认后端 token 里有多余的 `Bearer ` 后，决定**把后端的 `Bearer ` 删除**，只返回纯 token。前端 `authRequest` 自带的 `Bearer ` 拼接已经够用。

**改动位置：** `utils/utils.go` — `GenerateJWT()` 函数

```go
// 改前：return "Bearer " + signedToken, err
// 改后：
func GenerateJWT(username string) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "username": username,
        "exp":      time.Now().Add(time.Hour * 72).Unix(),
    })
    signedToken, err := token.SignedString([]byte(jwtSecret))
    return signedToken, err  // 只返回纯 JWT
}
```

## 验证

- 重新编译启动后端
- Postman 再次请求登录 → token 不再带 `Bearer ` 前缀 ✓
- 前端清 cookie → 重新登录 → 创建/加入组队正常 ✓

## 经验

JWT 的 `Bearer ` 前缀不能在前后端各加一次。本项目统一由前端 `authRequest` 拼接，后端只负责生成和验证纯 token。
