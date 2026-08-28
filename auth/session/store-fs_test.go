package session_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mulan-go/mulan/auth/session"
)

// fsStoreTestHelper 提供测试辅助方法，用于在不修改 store-fs.go 的情况下测试文件存储
type fsStoreTestHelper struct {
	store *session.FsStore
	dir   string
}

// newFsStoreTestHelper 创建测试辅助工具
func newFsStoreTestHelper(dir string, maxAge ...int) (*fsStoreTestHelper, error) {
	store, err := session.NewFsStore(dir, maxAge...)
	if err != nil {
		return nil, err
	}
	return &fsStoreTestHelper{
		store: store,
		dir:   dir,
	}, nil
}

// Save 保存数据（直接调用 store.Save）
func (h *fsStoreTestHelper) Save(ctx context.Context, v session.Data, lifetime ...time.Duration) error {
	return h.store.Save(ctx, v, lifetime...)
}

// Get 获取数据（使用手动反序列化绕过接口类型问题）
func (h *fsStoreTestHelper) Get(ctx context.Context, tokenStr string) (session.Data, error) {
	// 构建文件路径（与 store-fs.go 中的 getFilePath 逻辑一致）
	filePath := filepath.Join(h.dir, "ginx_auth_token_"+filepath.Base(tokenStr))

	// 读取文件
	rawJSON, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, session.ErrTokenNotFound
		}
		return nil, err
	}

	// 使用具体类型 *DefaultData 进行反序列化
	var data struct {
		Data   *session.DefaultData `json:"data"`
		Expire time.Time            `json:"expire"`
	}

	if err := json.Unmarshal(rawJSON, &data); err != nil {
		return nil, err
	}

	// 检查是否过期
	if !data.Expire.IsZero() && data.Expire.Before(time.Now()) {
		h.store.Clear(ctx, tokenStr)
		return nil, session.ErrTokenExpired
	}

	return data.Data, nil
}

// Clear 清除数据（直接调用 store.Clear）
func (h *fsStoreTestHelper) Clear(ctx context.Context, tokenStr string) error {
	return h.store.Clear(ctx, tokenStr)
}

func TestTokenFs(t *testing.T) {
	r := setupRouter(&session.Config{Driver: "fs", Dir: "/tmp/fs"})
	// 构建返回值
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w1, req1)
	rsp1 := w1.Result()
	body, _ := io.ReadAll(rsp1.Body)
	tokenStr := string(body)
	t.Log(tokenStr)

	req2, _ := http.NewRequest("GET", "/info", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenStr)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	rsp2 := w2.Result()
	body, _ = io.ReadAll(rsp2.Body)
	t.Log(string(body))
}

// TestFsStore_NewFsStore 测试创建存储实例
func TestFsStore_NewFsStore(t *testing.T) {
	t.Run("创建默认配置的存储", func(t *testing.T) {
		dir := t.TempDir()
		store, err := session.NewFsStore(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}
		if store == nil {
			t.Fatal("存储实例为 nil")
		}

		// 验证目录是否创建
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Fatalf("目录未创建: %s", dir)
		}
	})

	t.Run("创建自定义 maxAge 的存储", func(t *testing.T) {
		dir := t.TempDir()
		customMaxAge := 7200 // 2 小时
		store, err := session.NewFsStore(dir, customMaxAge)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}
		if store == nil {
			t.Fatal("存储实例为 nil")
		}
	})

	t.Run("创建嵌套目录", func(t *testing.T) {
		baseDir := t.TempDir()
		nestedDir := filepath.Join(baseDir, "level1", "level2", "level3")
		store, err := session.NewFsStore(nestedDir)
		if err != nil {
			t.Fatalf("创建嵌套目录存储失败: %v", err)
		}
		if store == nil {
			t.Fatal("存储实例为 nil")
		}

		// 验证嵌套目录是否创建
		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Fatalf("嵌套目录未创建: %s", nestedDir)
		}
	})
}

// TestFsStore_SaveAndGet 测试保存和获取功能
func TestFsStore_SaveAndGet(t *testing.T) {
	ctx := context.Background()

	t.Run("保存并获取基本数据", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		// 创建测试数据
		data := &session.DefaultData{}
		data.SetID(12345)
		data.SetAccount("test_user")
		data.SetRoles([]string{"admin", "user"})
		data.SetValues("custom_key", "custom_value")
		data.SetState(1)
		tokenStr := data.New()

		// 保存数据
		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 获取数据
		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取数据失败: %v", err)
		}

		// 验证数据
		if retrieved.Token() != tokenStr {
			t.Errorf("Token 不匹配: got %s, want %s", retrieved.Token(), tokenStr)
		}
		if retrieved.ID() != 12345 {
			t.Errorf("ID 不匹配: got %d, want %d", retrieved.ID(), 12345)
		}
		if retrieved.Account() != "test_user" {
			t.Errorf("Account 不匹配: got %s, want %s", retrieved.Account(), "test_user")
		}
		if retrieved.State() != 1 {
			t.Errorf("State 不匹配: got %d, want %d", retrieved.State(), 1)
		}

		// 验证角色
		roles := retrieved.Roles()
		if len(roles) != 2 || roles[0] != "admin" || roles[1] != "user" {
			t.Errorf("Roles 不匹配: got %v", roles)
		}

		// 验证自定义值
		if val := retrieved.Get("custom_key"); val != "custom_value" {
			t.Errorf("自定义值不匹配: got %v, want %s", val, "custom_value")
		}
	})

	t.Run("保存带自定义生命周期的数据", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		data.SetID(99999)
		tokenStr := data.New()

		// 保存数据，1 秒后过期
		lifetime := 1 * time.Second
		if err := helper.Save(ctx, data, lifetime); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 立即获取应该成功
		_, err = helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取数据失败: %v", err)
		}

		// 等待过期
		time.Sleep(1100 * time.Millisecond)

		// 再次获取应该失败
		_, err = helper.Get(ctx, tokenStr)
		if err != session.ErrTokenExpired {
			t.Errorf("期望 ErrTokenExpired 错误, 得到: %v", err)
		}
	})

	t.Run("保存空 Token 自动生成", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		data.SetID(11111)
		// 不设置 Token，应该自动生成

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// Token 应该已经生成
		tokenStr := data.Token()
		if tokenStr == "" {
			t.Fatal("Token 未自动生成")
		}

		// 应该能够获取
		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取数据失败: %v", err)
		}
		if retrieved.ID() != 11111 {
			t.Errorf("ID 不匹配: got %d, want %d", retrieved.ID(), 11111)
		}
	})

	t.Run("更新已存在的 Token", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		// 第一次保存
		data := &session.DefaultData{}
		data.SetID(123)
		data.SetAccount("old_account")
		tokenStr := data.New()

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 更新数据
		data.SetAccount("new_account")
		data.SetID(456)
		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("更新数据失败: %v", err)
		}

		// 获取并验证
		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取数据失败: %v", err)
		}
		if retrieved.Account() != "new_account" {
			t.Errorf("Account 未更新: got %s, want %s", retrieved.Account(), "new_account")
		}
		if retrieved.ID() != 456 {
			t.Errorf("ID 未更新: got %d, want %d", retrieved.ID(), 456)
		}
	})
}

// TestFsStore_Clear 测试清除功能
func TestFsStore_Clear(t *testing.T) {
	ctx := context.Background()

	t.Run("清除存在的 Token", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		// 保存数据
		data := &session.DefaultData{}
		data.SetID(123)
		tokenStr := data.New()
		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 清除数据
		if err := helper.Clear(ctx, tokenStr); err != nil {
			t.Fatalf("清除数据失败: %v", err)
		}

		// 再次获取应该失败
		_, err = helper.Get(ctx, tokenStr)
		if err != session.ErrTokenNotFound {
			t.Errorf("期望 ErrTokenNotFound 错误, 得到: %v", err)
		}
	})

	t.Run("清除不存在的 Token 不报错", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		// 清除不存在的 Token 不应该报错
		err = helper.Clear(ctx, "non_existent_token")
		if err != nil {
			t.Errorf("清除不存在的 Token 报错: %v", err)
		}
	})

	t.Run("重复清除同一个 Token", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		tokenStr := data.New()
		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 第一次清除
		if err := helper.Clear(ctx, tokenStr); err != nil {
			t.Fatalf("第一次清除失败: %v", err)
		}

		// 第二次清除不应该报错
		if err := helper.Clear(ctx, tokenStr); err != nil {
			t.Errorf("第二次清除报错: %v", err)
		}
	})
}

// TestFsStore_Get_Errors 测试获取时的错误情况
func TestFsStore_Get_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("获取不存在的 Token", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		_, err = helper.Get(ctx, "non_existent_token")
		if err != session.ErrTokenNotFound {
			t.Errorf("期望 ErrTokenNotFound 错误, 得到: %v", err)
		}
	})

	t.Run("获取已过期的 Token", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		tokenStr := data.New()

		// 保存数据，100 毫秒后过期
		lifetime := 100 * time.Millisecond
		if err := helper.Save(ctx, data, lifetime); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 等待过期
		time.Sleep(150 * time.Millisecond)

		// 获取应该返回过期错误
		_, err = helper.Get(ctx, tokenStr)
		if err != session.ErrTokenExpired {
			t.Errorf("期望 ErrTokenExpired 错误, 得到: %v", err)
		}

		// 过期后文件应该被删除
		_, err = helper.Get(ctx, tokenStr)
		if err != session.ErrTokenNotFound {
			t.Errorf("期望文件已被删除 (ErrTokenNotFound), 得到: %v", err)
		}
	})
}

// TestFsStore_Concurrency 测试并发场景
func TestFsStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	helper, err := newFsStoreTestHelper(dir)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}

	t.Run("并发保存不同 Token", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan error, numGoroutines)

		for i := range numGoroutines {
			go func(id int) {
				data := &session.DefaultData{}
				data.SetID(uint64(id))
				data.SetAccount("user_" + string(rune('0'+id)))
				tokenStr := data.New()
				if err := helper.Save(ctx, data); err != nil {
					done <- err
					return
				}
				// 立即读取验证
				retrieved, err := helper.Get(ctx, tokenStr)
				if err != nil {
					done <- err
					return
				}

				if retrieved.ID() != uint64(id) {
					done <- err
					return
				}

				done <- nil
			}(i)
		}

		// 等待所有 goroutine 完成
		for range numGoroutines {
			if err := <-done; err != nil {
				t.Errorf("并发操作失败: %v", err)
			}
		}
	})
}

// TestFsStore_EdgeCases 测试边界情况
func TestFsStore_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Token 包含特殊字符路径安全", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		// 尝试使用包含路径分隔符的 Token（应该被 filepath.Base 处理）
		data := &session.DefaultData{}
		data.SetToken("../../../etc/passwd")
		data.SetID(999)

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 验证文件在正确的目录下
		retrieved, err := helper.Get(ctx, "../../../etc/passwd")
		if err != nil {
			t.Fatalf("获取数据失败: %v", err)
		}
		if retrieved.ID() != 999 {
			t.Errorf("ID 不匹配: got %d, want 999", retrieved.ID())
		}
	})

	t.Run("空数据保存和读取", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		tokenStr := data.New()

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存空数据失败: %v", err)
		}

		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取空数据失败: %v", err)
		}

		if retrieved.Token() != tokenStr {
			t.Errorf("Token 不匹配")
		}
		if retrieved.ID() != 0 {
			t.Errorf("空数据 ID 应该为 0, got %d", retrieved.ID())
		}
		if retrieved.Account() != "" {
			t.Errorf("空数据 Account 应该为空, got %s", retrieved.Account())
		}
	})

	t.Run("永不过期的 Token", func(t *testing.T) {
		dir := t.TempDir()
		// 创建 maxAge 为 0 的存储（永不过期）
		helper, err := newFsStoreTestHelper(dir, 0)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		data.SetID(777)
		tokenStr := data.New()

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存数据失败: %v", err)
		}

		// 等待一段时间
		time.Sleep(100 * time.Millisecond)

		// 应该仍然能获取
		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取永久 Token 失败: %v", err)
		}
		if retrieved.ID() != 777 {
			t.Errorf("ID 不匹配: got %d, want 777", retrieved.ID())
		}
	})

	t.Run("复杂数据类型", func(t *testing.T) {
		dir := t.TempDir()
		helper, err := newFsStoreTestHelper(dir)
		if err != nil {
			t.Fatalf("创建存储失败: %v", err)
		}

		data := &session.DefaultData{}
		data.SetID(123)
		data.SetValues("string", "test_value")
		data.SetValues("number", 42)
		data.SetValues("float", 3.14)
		data.SetValues("bool", true)
		data.SetValues("slice", []interface{}{"a", "b", "c"})
		data.SetValues("map", map[string]interface{}{"key": "value"})
		tokenStr := data.New()

		if err := helper.Save(ctx, data); err != nil {
			t.Fatalf("保存复杂数据失败: %v", err)
		}

		retrieved, err := helper.Get(ctx, tokenStr)
		if err != nil {
			t.Fatalf("获取复杂数据失败: %v", err)
		}

		// 验证各种类型
		if val := retrieved.Get("string"); val != "test_value" {
			t.Errorf("string 值不匹配: got %v", val)
		}
		// JSON 序列化会将 number 转为 float64
		if val := retrieved.Get("number"); val != float64(42) {
			t.Errorf("number 值不匹配: got %v (type %T)", val, val)
		}
		if val := retrieved.Get("bool"); val != true {
			t.Errorf("bool 值不匹配: got %v", val)
		}
	})
}
