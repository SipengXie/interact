package mis

import (
	"fmt"
	"sync"
	"testing"

	"github.com/panjf2000/ants/v2"
)

func taskFuncWrapper(ch chan int, i int, wg *sync.WaitGroup) func() {
	return func() {
		ch <- i
		wg.Done()
	}
}

func TestAntsArray(t *testing.T) {
	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup
	ch := make(chan int, 10)

	antsWG.Add(10)
	for i := 0; i < 10; i++ {
		err := antsPool.Submit(taskFuncWrapper(ch, i, &antsWG))
		if err != nil {
			panic(err)
		}
	}
	antsWG.Wait()
	close(ch)
	for data := range ch {
		fmt.Println(data)
	}
}
