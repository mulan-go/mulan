package session_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mulan-go/mulan/auth/session"
	"github.com/redis/go-redis/v9"
)

// getRedisClient 获取Redis客户端（用于测试）
func getRedisClient() (redis.UniversalClient, bool) {
	// 从环境变量获取Redis地址
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, false
	}

	return client, true
}

// TestRedisStore 测试RedisStore基本功能
func TestRedisStore(t *testing.T) {
	client, ok := getRedisClient()
	if !ok {
		t.Skip("Redis not available, skipping test")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		t.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	// 测试保存和获取
	t.Run("SaveAndGet", func(t *testing.T) {
		data := &session.DefaultData{Token_: "test_token_001"}
		data.SetID(1)
		data.SetAccount("test_user")
		data.SetRoles([]string{"admin", "user"})
		data.SetValues("custom_key", "custom_value")

		// 保存
		err := store.Save(ctx, data)
		if err != nil {
			t.Fatal("Failed to save:", err)
		}

		// 获取
		retrievedData, err := store.Get(ctx, "test_token_001")
		if err != nil {
			t.Fatal("Failed to get:", err)
		}

		// 验证
		if retrievedData.ID() != 1 {
			t.Errorf("Expected ID 1, got %d", retrievedData.ID())
		}
		if retrievedData.Account() != "test_user" {
			t.Errorf("Expected account 'test_user', got '%s'", retrievedData.Account())
		}

		// 清理
		store.Clear(ctx, "test_token_001")
	})

	// 测试过期时间
	t.Run("Expiration", func(t *testing.T) {
		data := &session.DefaultData{Token_: "test_token_002"}
		data.SetID(2)

		// 保存，1秒过期
		err := store.Save(ctx, data, 1*time.Second)
		if err != nil {
			t.Fatal("Failed to save:", err)
		}

		// 立即获取应该成功
		_, err = store.Get(ctx, "test_token_002")
		if err != nil {
			t.Fatal("Should be able to get immediately:", err)
		}

		// 等待过期
		time.Sleep(2 * time.Second)

		// 获取应该失败
		_, err = store.Get(ctx, "test_token_002")
		if err != session.ErrTokenNotFound {
			t.Error("Token should have expired")
		}
	})

	// 测试清除
	t.Run("Clear", func(t *testing.T) {
		data := &session.DefaultData{Token_: "test_token_003"}
		data.SetID(3)

		// 保存
		err := store.Save(ctx, data)
		if err != nil {
			t.Fatal("Failed to save:", err)
		}

		// 清除
		err = store.Clear(ctx, "test_token_003")
		if err != nil {
			t.Fatal("Failed to clear:", err)
		}

		// 获取应该失败
		_, err = store.Get(ctx, "test_token_003")
		if err != session.ErrTokenNotFound {
			t.Error("Token should be cleared")
		}
	})

	// 测试不存在的token
	t.Run("NonExistentToken", func(t *testing.T) {
		_, err := store.Get(ctx, "non_existent_token")
		if err != session.ErrTokenNotFound {
			t.Error("Should return ErrTokenNotFound for non-existent token")
		}
	})

	// 测试更新
	t.Run("Update", func(t *testing.T) {
		data := &session.DefaultData{Token_: "test_token_004"}
		data.SetID(4)
		data.SetAccount("original")

		// 保存
		err := store.Save(ctx, data)
		if err != nil {
			t.Fatal("Failed to save:", err)
		}

		// 更新
		data.SetAccount("updated")
		err = store.Save(ctx, data)
		if err != nil {
			t.Fatal("Failed to update:", err)
		}

		// 获取并验证
		retrievedData, err := store.Get(ctx, "test_token_004")
		if err != nil {
			t.Fatal("Failed to get:", err)
		}

		if retrievedData.Account() != "updated" {
			t.Errorf("Expected account 'updated', got '%s'", retrievedData.Account())
		}

		// 清理
		store.Clear(ctx, "test_token_004")
	})
}

// TestRedisStoreWithSession 测试RedisStore与Session集成
func TestRedisStoreWithSession(t *testing.T) {
	client, ok := getRedisClient()
	if !ok {
		t.Skip("Redis not available, skipping test")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		t.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()
	data := &session.DefaultData{Token_: "session_test_token"}

	sess := session.NewSession(ctx, store, data)

	// 设置数据
	sess.SetID(100)
	sess.SetAccount("session_user")
	sess.SetRoles([]string{"admin"})
	sess.SetValues("key1", "value1")

	// 保存
	err = sess.Save()
	if err != nil {
		t.Fatal("Failed to save session:", err)
	}

	// 创建新Session，从Redis加载
	newData := &session.DefaultData{Token_: "session_test_token"}
	newSess := session.NewSession(ctx, store, newData)

	// 验证数据
	if newSess.ID() != 100 {
		t.Errorf("Expected ID 100, got %d", newSess.ID())
	}
	if newSess.Account() != "session_user" {
		t.Errorf("Expected account 'session_user', got '%s'", newSess.Account())
	}
	if !newSess.HasRole("admin") {
		t.Error("Expected to have admin role")
	}

	// 清理
	store.Clear(ctx, "session_test_token")
}

// BenchmarkRedisStoreSave 基准测试：保存
func BenchmarkRedisStoreSave(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := &session.DefaultData{Token_: fmt.Sprintf("bench_token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	// 清理
	b.StopTimer()
	for i := 0; i < b.N && i < 1000; i++ {
		store.Clear(ctx, fmt.Sprintf("bench_token_%d", i))
	}
}

// BenchmarkRedisStoreGet 基准测试：获取
func BenchmarkRedisStoreGet(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	// 准备测试数据
	data := &session.DefaultData{Token_: "bench_get_token"}
	data.SetID(1)
	data.SetAccount("test_user")
	store.Save(ctx, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(ctx, "bench_get_token")
	}

	// 清理
	b.StopTimer()
	store.Clear(ctx, "bench_get_token")
}

// BenchmarkRedisStoreGetParallel 基准测试：并发获取
func BenchmarkRedisStoreGetParallel(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	// 准备100个测试token
	for i := 0; i < 100; i++ {
		data := &session.DefaultData{Token_: fmt.Sprintf("parallel_token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tokenStr := fmt.Sprintf("parallel_token_%d", i%100)
			_, _ = store.Get(ctx, tokenStr)
			i++
		}
	})

	// 清理
	b.StopTimer()
	for i := 0; i < 100; i++ {
		store.Clear(ctx, fmt.Sprintf("parallel_token_%d", i))
	}
}

// BenchmarkRedisStoreSaveParallel 基准测试：并发保存
func BenchmarkRedisStoreSaveParallel(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			data := &session.DefaultData{Token_: fmt.Sprintf("parallel_save_%d", i)}
			data.SetID(uint64(i))
			data.SetAccount(fmt.Sprintf("user_%d", i))
			store.Save(ctx, data)
			i++
		}
	})
}

// BenchmarkRedisStoreMixed 基准测试：混合读写（80%读，20%写）
func BenchmarkRedisStoreMixed(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	// 准备100个测试token
	for i := 0; i < 100; i++ {
		data := &session.DefaultData{Token_: fmt.Sprintf("mixed_token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%5 == 0 { // 20%写操作
				data := &session.DefaultData{Token_: fmt.Sprintf("mixed_token_%d", i%100)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
			} else { // 80%读操作
				tokenStr := fmt.Sprintf("mixed_token_%d", i%100)
				_, _ = store.Get(ctx, tokenStr)
			}
			i++
		}
	})

	// 清理
	b.StopTimer()
	for i := 0; i < 100; i++ {
		store.Clear(ctx, fmt.Sprintf("mixed_token_%d", i))
	}
}

// BenchmarkStoreComparison_Redis 对比不同Store实现（包含Redis）
func BenchmarkStoreComparison_Redis(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	ctx := context.Background()

	stores := map[string]session.Store{
		"MemStore":        session.NewMemStore(),
		"ShardedMemStore": session.NewShardedMemStore(16),
	}

	redisStore, err := session.NewRedisStore(client)
	if err == nil {
		stores["RedisStore"] = redisStore
	}

	for name, store := range stores {
		b.Run(name+"_Save", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				data := &session.DefaultData{Token_: fmt.Sprintf("compare_token_%d", i)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
			}
		})

		// 预先保存一个token用于Get测试
		testData := &session.DefaultData{Token_: "compare_test_token"}
		testData.SetID(1)
		store.Save(ctx, testData)

		b.Run(name+"_Get", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = store.Get(ctx, "compare_test_token")
			}
		})
	}
}

// BenchmarkRedisStoreWithPipeline 测试Pipeline优化效果
func BenchmarkRedisStoreWithPipeline(b *testing.B) {
	client, ok := getRedisClient()
	if !ok {
		b.Skip("Redis not available, skipping benchmark")
		return
	}
	defer client.Close()

	store, err := session.NewRedisStore(client)
	if err != nil {
		b.Fatal("Failed to create RedisStore:", err)
	}

	ctx := context.Background()

	b.Run("WithExpiration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data := &session.DefaultData{Token_: fmt.Sprintf("pipeline_token_%d", i)}
			data.SetID(uint64(i))
			store.Save(ctx, data, 3600*time.Second)
		}
	})

	b.Run("WithoutExpiration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data := &session.DefaultData{Token_: fmt.Sprintf("pipeline_token_%d", i)}
			data.SetID(uint64(i))
			store.Save(ctx, data)
		}
	})
}
