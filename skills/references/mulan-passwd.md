# Mulan Passwd - 密码工具

```go
import "github.com/mulan-go/mulan/passwd"
```

`mulan-passwd` 提供常用的密码处理能力，适合放在注册、登录、重置密码这类场景中使用。

主要能力：

- 密码加密
- 密码校验
- 密码强度检查
- 随机密码生成

---

## 安装

```bash
go get github.com/mulan-go/mulan/passwd
```

---

## API

### 1. 密码加密

```go
passwd.Encrypt(password string) string
```

说明：

- 使用 `bcrypt` 生成密码哈希
- 返回值是加密后的字符串
- 如果内部失败，会返回空字符串

示例：

```go
hash := passwd.Encrypt("123456")
if hash == "" {
    panic("encrypt password failed")
}
```

---

### 2. 密码校验

```go
passwd.Verify(hash, password string) bool
```

说明：

- 用明文密码校验已有哈希
- 返回 `true` 表示密码正确
- 返回 `false` 表示密码不匹配

示例：

```go
ok := passwd.Verify(hash, "123456")
if !ok {
    // 密码错误
}
```

---

### 3. 密码强度检查

```go
passwd.Check(password string) (Strength, Combination)
```

返回两个值：

- `Strength`：密码强度
- `Combination`：密码包含的字符组合类型

示例：

```go
strength, combo := passwd.Check("Abc123!@#")
_ = combo

if strength < passwd.Medium {
    // 密码太弱
}
```

---

### 4. 随机密码生成

```go
passwd.Generate(length int, c Combination) string
```

说明：

- `length`：密码长度
- `c`：允许使用的字符组合

示例：

```go
pwd := passwd.Generate(12, passwd.Word)
pwd2 := passwd.Generate(16, passwd.All)
```

---

## 类型定义

### `Strength`

```go
type Strength int
```

强度枚举：

```go
const (
    Weak Strength = iota
    Low
    Medium
    High
)
```

含义：

- `Weak`：弱
- `Low`：较弱
- `Medium`：中等
- `High`：较强

---

### `Combination`

```go
type Combination int
```

字符组合枚举：

```go
const (
    Lowercase Combination = 1 << iota
    Uppercase
    Number
    Special

    Alpha = Lowercase | Uppercase
    Word  = Alpha | Number
    All   = Word | Special
)
```

说明：

- `Lowercase`：小写字母
- `Uppercase`：大写字母
- `Number`：数字
- `Special`：特殊字符
- `Alpha`：大小写字母
- `Word`：字母 + 数字
- `All`：字母 + 数字 + 特殊字符

---

## 常见用法

### 用户注册

```go
func handleRegister(c *gin.Context) {
    var _req RegisterReq
    if err := req.Bind(c, &_req); err != nil {
        c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
        return
    }

    strength, _ := passwd.Check(_req.Password)
    if strength < passwd.Medium {
        c.JSON(400, rsp.M("密码强度过低"))
        return
    }

    hash := passwd.Encrypt(_req.Password)
    if hash == "" {
        c.JSON(500, rsp.M("密码加密失败"))
        return
    }

    // 保存 hash 到数据库
    _ = hash

    c.JSON(200, rsp.OK())
}
```

---

### 用户登录

```go
func handleLogin(c *gin.Context) {
    var _req LoginReq
    if err := req.Bind(c, &_req); err != nil {
        c.JSON(400, rsp.E(rsp.ErrParamInvalid.WithError(err)))
        return
    }

    // 假设从数据库查到用户密码哈希
    userHash := "$2a$10$example"

    if !passwd.Verify(userHash, _req.Password) {
        c.JSON(400, rsp.M("账号或密码错误"))
        return
    }

    c.JSON(200, rsp.OK())
}
```

---

### 重置密码

```go
func handleResetPassword(c *gin.Context) {
    tempPassword := passwd.Generate(12, passwd.Word)

    hash := passwd.Encrypt(tempPassword)
    if hash == "" {
        c.JSON(500, rsp.M("临时密码生成失败"))
        return
    }

    // 保存 hash
    _ = hash

    c.JSON(200, rsp.S(gin.H{
        "temp_password": tempPassword,
    }))
}
```

---

### 生成高强度密码

```go
pwd := passwd.Generate(16, passwd.All)
```

适合：

- 初始化管理员密码
- 临时密码
- 邀请码式账号初始密码

---

## 强度检查说明

`passwd.Check(...)` 会统计密码中是否包含这些字符类型：

- 小写字母
- 大写字母
- 数字
- 特殊字符

密码包含的类型越多，强度越高。

例如：

```go
strength, combo := passwd.Check("abc123")
_ = combo
// 一般会得到较低到中等强度
_ = strength
```

```go
strength, combo := passwd.Check("Abc123!@#")
_ = combo
// 一般会更高
_ = strength
```

---

## 生成密码说明

`passwd.Generate(length, c)` 会按给定组合随机生成密码。

例子：

```go
passwd.Generate(8, passwd.Number)   // 纯数字
passwd.Generate(12, passwd.Word)    // 字母+数字
passwd.Generate(16, passwd.All)     // 字母+数字+特殊字符
passwd.Generate(10, passwd.Alpha)   // 纯字母
```

注意：

- 如果需要更强密码，优先使用 `passwd.All`
- 长度建议不少于 `12`
- 管理员或系统密码建议 `16` 位以上

---

## 推荐实践

1. **数据库只保存哈希，不保存明文**
2. **注册时先检查强度，再加密保存**
3. **登录时使用 `Verify` 校验，不要自己比对字符串**
4. **重置密码后立即返回一次，并提示用户尽快修改**
5. **建议配合 `mulan-rsp` 返回统一响应**
6. **建议配合 `mulan-req` 处理请求绑定**

---

## 常见组合

### 注册流程

```go
strength, _ := passwd.Check(req.Password)
if strength < passwd.Medium {
    return rsp.M("密码太弱")
}

hash := passwd.Encrypt(req.Password)
if hash == "" {
    return rsp.M("密码加密失败")
}
```

### 登录流程

```go
if !passwd.Verify(user.Password, req.Password) {
    return rsp.M("账号或密码错误")
}
```

### 临时密码流程

```go
tempPassword := passwd.Generate(16, passwd.All)
hash := passwd.Encrypt(tempPassword)
```

---

## 小结

`mulan-passwd` 很适合在账号系统里直接使用：

- `Encrypt`：保存前加密
- `Verify`：登录时校验
- `Check`：判断密码强度
- `Generate`：生成随机密码

如果你在 Mulan 项目里做认证模块，通常可以这样组合：

- `mulan-req`：绑定请求参数
- `mulan-rsp`：统一返回结果
- `mulan-passwd`：处理密码
- `mulan-auth`：处理登录态与会话
