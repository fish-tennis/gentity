package examples

import (
	"github.com/fish-tennis/gentity"
	"sync"
	"testing"
)

// mongo实现的自增id方式
func TestIncrementId(t *testing.T) {
	mongoDb := gentity.NewMongoDb(_mongoUri, _mongoDbName)
	kvDb := mongoDb.RegisterKvDb("kv", "k", "v")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	defer func() {
		mongoDb.Disconnect()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val,err := kvDb.Inc("id", 1, true)
			if err != nil {
				t.Logf("%v", err)
			}
			t.Logf("%v", val)
		}()
	}
	wg.Wait()
}
