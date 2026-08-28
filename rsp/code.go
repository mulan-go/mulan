package rsp

type (
	SystemCode int
	BizCode    int
)

const (
	// 系统代码
	_ok      SystemCode = iota
	Client              // 客户请求错误
	Internal            // 内部错误
	Remote              // 远程错误
	// 业务代码
	_biz_code    BizCode = iota + 1000
	Auth                 // Auth
	Captcha              // Captcha
	Database             // Database
	File                 // File
	RemoteConfig         // RemoteConfig
)

var (
	Success          = New(_ok, 0, 0, "成功")
	ErrUnknownErr    = New(Internal, 0, 0, "未知错误")
	ErrUnImplemented = New(Internal, 0, 1, "未实现")
	ErrNotFound      = New(Internal, 0, 2, "资源未找到")
	// Request
	ErrBadRequest   = New(Client, 0, 0, "请求不合法")
	ErrRequestLimit = New(Client, 0, 1, "请求过于频繁")
	ErrParamInvalid = New(Client, 0, 2, "参数不合法")
)

var (
	DatabaseUnknown  = New(Internal, Database, 0, "未知数据库错误")
	DatabaseExists   = New(Internal, Database, 1, "已存在")
	DatabaseUpdate   = New(Internal, Database, 2, "更新失败")
	DatabaseDelete   = New(Internal, Database, 3, "删除失败")
	DatabaseInsert   = New(Internal, Database, 4, "插入失败")
	DatabaseTruncate = New(Internal, Database, 5, "清空失败")
)

// Auth
var (
	Unauthorized     = New(Client, Auth, 1, "未授权")  // Token Not Found in Request
	Forbidden        = New(Client, Auth, 2, "权限不足") // Permission Denied || Banned
	TokenInvalid     = New(Client, Auth, 3, "令牌无效")
	TokenExpired     = New(Client, Auth, 4, "令牌过期") // Token Not Found in Store
	PasswordInvalid  = New(Client, Auth, 5, "密码错误")
	PasswordWeak     = New(Client, Auth, 6, "密码强度不足")
	PasswordMatch    = New(Client, Auth, 7, "密码不匹配")
	PasswordReset    = New(Client, Auth, 8, "密码重置失败")
	EmailDisable     = New(Client, Auth, 9, "邮箱未开启")
	EmailCodeInvalid = New(Client, Auth, 10, "邮箱验证码无效")

	TokenCreate     = New(Internal, Auth, 1, "令牌创建失败")
	TokenDestory    = New(Internal, Auth, 2, "令牌销毁失败")
	RegisterExists  = New(Internal, Auth, 3, "用户已存在")
	RegisterFailed  = New(Internal, Auth, 4, "用户注册失败")
	RegisterDisable = New(Internal, Auth, 5, "用户注册已关闭")

	EmailCodeSend = New(Remote, Auth, 1, "邮箱验证码发送失败")
)

// Captcha
var (
	CaptchaInvalid  = New(Client, Captcha, 1, "验证码错误")
	CaptchaGenerate = New(Internal, Captcha, 1, "验证码生成失败")
)

// File
var (
	FileSize = New(Client, File, 1, "文件大小超出限制")
	FileType = New(Client, File, 2, "文件类型不支持")

	FileUpload = New(Internal, File, 1, "文件上传失败")
	FileOpen   = New(Internal, File, 2, "文件打开失败")
	FileParse  = New(Internal, File, 3, "文件解析失败")
	FileExport = New(Internal, File, 4, "文件导出失败")
)
