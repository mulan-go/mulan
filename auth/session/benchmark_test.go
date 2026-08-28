package session_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/mulan-go/mulan/auth/session"
)

// BenchmarkDataNew 测试Data.New()性能
func BenchmarkDataNew(b *testing.B) {
	data := &session.DefaultData{}
	b.ResetTimer()
	for b.Loop() {
		data.New()
	}
}

// BenchmarkDataSetGet 测试Data的Set/Get性能
func BenchmarkDataSetGet(b *testing.B) {
	data := &session.DefaultData{}
	b.ResetTimer()
	for b.Loop() {
		data.Set("key", "value")
		_ = data.Get("key")
	}
}

// BenchmarkMemStoreSave 测试MemStore保存性能
func BenchmarkMemStoreSave(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test")

	b.ResetTimer()
	for b.Loop() {
		store.Save(ctx, data)
	}
}

// BenchmarkMemStoreGet 测试MemStore读取性能
func BenchmarkMemStoreGet(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test")
	store.Save(ctx, data)

	b.ResetTimer()
	for b.Loop() {
		_, _ = store.Get(ctx, "test_token")
	}
}

// BenchmarkMemStoreGetParallel 测试MemStore并发读取性能（读写锁优化的关键）
func BenchmarkMemStoreGetParallel(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()

	// 预先存储100个token
	for i := range 100 {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			token := fmt.Sprintf("token_%d", i%100)
			_, _ = store.Get(ctx, token)
			i++
		}
	})
}

// BenchmarkMemStoreSaveParallel 测试MemStore并发写入性能
func BenchmarkMemStoreSaveParallel(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
			data.SetID(uint64(i))
			data.SetAccount(fmt.Sprintf("user_%d", i))
			store.Save(ctx, data)
			i++
		}
	})
}

// BenchmarkMemStoreMixed 测试MemStore混合读写性能（80%读，20%写）
func BenchmarkMemStoreMixed(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	// 预先存储100个token
	for i := range 100 {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%5 == 0 { // 20%写操作
				data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i%100)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
			} else { // 80%读操作
				token := fmt.Sprintf("token_%d", i%100)
				_, _ = store.Get(ctx, token)
			}
			i++
		}
	})
}

// BenchmarkSessionData 测试Session.Data()懒加载性能
func BenchmarkSessionData(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test")
	store.Save(ctx, data)

	sess := session.NewSession(ctx, store, data)

	b.ResetTimer()
	for b.Loop() {
		_ = sess.Data()
	}
}

// BenchmarkSessionDataParallel 测试Session.Data()并发访问性能
func BenchmarkSessionDataParallel(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test")
	store.Save(ctx, data)

	sess := session.NewSession(ctx, store, data)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = sess.Data()
		}
	})
}

// BenchmarkSessionSetSave 测试Session设置和保存性能
func BenchmarkSessionSetSave(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	sess := session.NewSession(ctx, store, data)

	b.ResetTimer()
	for i := range b.N {
		sess.SetID(uint64(i))
		sess.SetAccount(fmt.Sprintf("user_%d", i))
		sess.SetRoles([]string{"user", "admin"})
		sess.Save()
	}
}

// BenchmarkFsStoreSave 测试FsStore保存性能
func BenchmarkFsStoreSave(b *testing.B) {
	store, err := session.NewFsStore("/tmp/benchmark_fs")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		// 清理测试文件
		// os.RemoveAll("/tmp/benchmark_fs")
	}()

	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}
}

// BenchmarkFsStoreGet 测试FsStore读取性能
func BenchmarkFsStoreGet(b *testing.B) {
	store, err := session.NewFsStore("/tmp/benchmark_fs")
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test")
	store.Save(ctx, data)

	b.ResetTimer()
	for b.Loop() {
		_, _ = store.Get(ctx, "test_token")
	}
}

// BenchmarkTokenGeneration 测试Token生成性能
func BenchmarkTokenGeneration(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		_ = session.New()
	}
}

// BenchmarkHasRole 测试角色检查性能
func BenchmarkHasRole(b *testing.B) {
	data := &session.DefaultData{}
	data.SetRoles([]string{"user", "admin", "editor", "viewer"})
	sess := session.NewSession(context.Background(), session.NewMemStore(), data)

	b.ResetTimer()
	for b.Loop() {
		_ = sess.HasRole("admin")
	}
}

// BenchmarkConcurrentSessions 测试多个Session并发操作
func BenchmarkConcurrentSessions(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()

	// 创建100个session
	sessions := make([]*session.Session, 100)
	for i := range b.N {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
		sessions[i] = session.NewSession(ctx, store, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sess := sessions[i%100]
			_ = sess.Data()
			sess.SetID(uint64(i))
			i++
		}
	})
}

// BenchmarkMemoryAllocation 测试内存分配
func BenchmarkMemoryAllocation(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		data.SetRoles([]string{"user", "admin"})
		data.SetValues("custom", "value")

		sess := session.NewSession(ctx, store, data)
		sess.Save()
		_ = sess.Data()
	}
}

// BenchmarkDataJSONMarshal 测试JSON序列化性能
func BenchmarkDataJSONMarshal(b *testing.B) {
	data := &session.DefaultData{Token_: "test_token"}
	data.SetID(1)
	data.SetAccount("test_user")
	data.SetRoles([]string{"admin", "user"})
	data.SetValues("key1", "value1")
	data.SetValues("key2", "value2")

	b.ResetTimer()
	for b.Loop() {
		sess := session.NewSession(context.Background(), session.NewMemStore(), data)
		_, _ = sess.MarshalJSON()
	}
}

// BenchmarkExpiredTokenCleanup 测试过期token清理性能
func BenchmarkExpiredTokenCleanup(b *testing.B) {
	store := session.NewMemStore(1) // 1秒过期
	ctx := context.Background()

	// 预先创建大量token
	for i := range b.N {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	for i := range b.N {
		// 模拟获取和清理过期token
		_, _ = store.Get(ctx, fmt.Sprintf("token_%d", i%1000))
	}
}

// BenchmarkStoreComparison 对比不同Store实现的性能
func BenchmarkStoreComparison(b *testing.B) {
	stores := map[string]session.Store{
		"MemStore": session.NewMemStore(),
	}

	// 注意：FsStore性能测试需要IO，会比较慢
	fsStore, err := session.NewFsStore("/tmp/benchmark_comparison")
	if err == nil {
		stores["FsStore"] = fsStore
	}

	ctx := context.Background()

	for name, store := range stores {
		b.Run(name+"_Save", func(b *testing.B) {
			for i := range b.N {
				data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
			}
		})

		// 预先保存一个token用于Get测试
		testData := &session.DefaultData{Token_: "test_token"}
		testData.SetID(1)
		store.Save(ctx, testData)

		b.Run(name+"_Get", func(b *testing.B) {
			for b.Loop() {
				_, _ = store.Get(ctx, "test_token")
			}
		})
	}
}

// BenchmarkShardedMemStoreSaveParallel 测试分片Store并发写入性能
func BenchmarkShardedMemStoreSaveParallel(b *testing.B) {
	store := session.NewShardedMemStore(16) // 16个分片
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
			data.SetID(uint64(i))
			data.SetAccount(fmt.Sprintf("user_%d", i))
			store.Save(ctx, data)
			i++
		}
	})
}

// BenchmarkShardedMemStoreGetParallel 测试分片Store并发读取性能
func BenchmarkShardedMemStoreGetParallel(b *testing.B) {
	store := session.NewShardedMemStore(16)
	ctx := context.Background()

	// 预先存储100个token
	for i := 0; i < 100; i++ {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		data.SetID(uint64(i))
		data.SetAccount(fmt.Sprintf("user_%d", i))
		store.Save(ctx, data)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tokenStr := fmt.Sprintf("token_%d", i%100)
			_, _ = store.Get(ctx, tokenStr)
			i++
		}
	})
}

// BenchmarkShardedVsNormal 对比分片和普通Store性能
func BenchmarkShardedVsNormal(b *testing.B) {
	ctx := context.Background()

	b.Run("Normal_Parallel_Write", func(b *testing.B) {
		store := session.NewMemStore()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
				i++
			}
		})
	})

	b.Run("Sharded_Parallel_Write", func(b *testing.B) {
		store := session.NewShardedMemStore(16)
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
				data.SetID(uint64(i))
				store.Save(ctx, data)
				i++
			}
		})
	})
}

// BenchmarkSessionWithPool 测试使用对象池的Session性能
func BenchmarkSessionWithPool(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", i)}
		sess := session.NewSessionWithPool(ctx, store, data)
		sess.SetID(uint64(i))
		sess.SetAccount("user")
		sess.Save()
		session.ReleaseSession(sess)
	}
}

// BenchmarkRealWorldScenario 模拟真实业务场景
func BenchmarkRealWorldScenario(b *testing.B) {
	store := session.NewMemStore()
	ctx := context.Background()
	var mu sync.Mutex
	tokenCounter := 0

	// 模拟：10%登录，70%读取，20%更新
	b.RunParallel(func(pb *testing.PB) {
		localCounter := 0
		for pb.Next() {
			op := localCounter % 10

			switch {
			case op == 0: // 10% 登录（创建新session）
				mu.Lock()
				tokenCounter++
				tokenID := tokenCounter
				mu.Unlock()

				data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", tokenID)}
				data.SetID(uint64(tokenID))
				data.SetAccount(fmt.Sprintf("user_%d", tokenID))
				data.SetRoles([]string{"user"})
				store.Save(ctx, data)

			case op <= 7: // 70% 读取session
				mu.Lock()
				currentMax := tokenCounter
				mu.Unlock()

				if currentMax > 0 {
					tokenID := localCounter % currentMax
					if tokenID == 0 {
						tokenID = 1
					}
					_, _ = store.Get(ctx, fmt.Sprintf("token_%d", tokenID))
				}

			default: // 20% 更新session
				mu.Lock()
				currentMax := tokenCounter
				mu.Unlock()

				if currentMax > 0 {
					tokenID := localCounter % currentMax
					if tokenID == 0 {
						tokenID = 1
					}
					data := &session.DefaultData{Token_: fmt.Sprintf("token_%d", tokenID)}
					data.SetID(uint64(tokenID))
					data.SetAccount(fmt.Sprintf("updated_user_%d", tokenID))
					store.Save(ctx, data)
				}
			}

			localCounter++
		}
	})
}
