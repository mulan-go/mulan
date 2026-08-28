package session_test

import (
	"sync"
	"testing"

	"github.com/mulan-go/mulan/auth/session"
)

func TestDefaultDataConcurrency(t *testing.T) {
	data := &session.DefaultData{}
	var wg sync.WaitGroup
	numRoutines := 100
	numOps := 1000

	wg.Add(numRoutines)
	for i := range numRoutines {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				data.SetValues("key", j)
				data.SetID(uint64(j))
				_ = data.ID()
				_ = data.Items()
				data.Set("custom", j)
				_ = data.Get("custom")
			}
		}(i)
	}
	wg.Wait()
}

func TestDataRolesClone(t *testing.T) {
	data := &session.DefaultData{}
	roles := []string{"admin", "user"}
	data.SetRoles(roles)

	r1 := data.Roles()
	r1[0] = "hacker"

	r2 := data.Roles()
	if r2[0] == "hacker" {
		t.Error("Roles() returned a slice that allows modifying the internal state")
	}
}

func TestDataItemsClone(t *testing.T) {
	data := &session.DefaultData{}
	data.SetValues("foo", "bar")

	items1 := data.Items()
	items1["foo"] = "hacker"

	items2 := data.Items()
	if items2["foo"] == "hacker" {
		t.Error("Items() returned a map that allows modifying the internal state")
	}
}
