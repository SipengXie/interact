package utils

import (
	"fmt"
	"interact/state"
	"sync"

	"github.com/panjf2000/ants/v2"
)

// MergeToState merge all cacheState to fullstate.State
func MergeToState(cacheStates state.CacheStateList, db state.State) {
	for i := 0; i < len(cacheStates); i++ {
		cacheStates[i].MergeState(db)
	}
}

// MergeToStateConcurrent merge all cacheState to origin FullCacheConcurrent concurrently
func MergeToCacheStateConcurrent(pool *ants.Pool, cacheStates state.CacheStateList, db *state.FullCacheConcurrent, wg *sync.WaitGroup) {
	for i := 0; i < len(cacheStates); i++ {
		index := i
		err := pool.Submit(func() {
			cacheStates[index].MergeState(db)
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}
	}
	wg.Wait()
}
