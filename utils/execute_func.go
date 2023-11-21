package utils

import (
	"fmt"
	cachestate "interact/cacheState"
	"sync"

	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/panjf2000/ants/v2"
)

// MergeToState merge all cacheStateDB to origin stateDB
func MergeToState(cacheStates cachestate.CacheStateList, db *statedb.StateDB) {
	for i := 0; i < len(cacheStates); i++ {
		cacheStates[i].MergeState(db)
	}
}

// MergeToState merge all cacheStateDB to origin stateDB
func MergeToCacheState(cacheStates cachestate.CacheStateList, db *cachestate.CacheState) {
	for i := 0; i < len(cacheStates); i++ {
		cacheStates[i].MergeStateToCacheState(db)
	}
}

func MergeToCacheStateConcurrent(pool *ants.Pool, cacheStates cachestate.CacheStateList, db *cachestate.FullCacheConcurrent, wg *sync.WaitGroup) {
	for i := 0; i < len(cacheStates); i++ {
		index := i
		err := pool.Submit(func() {
			cacheStates[index].MergeStateToFullCache(db)
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}
	}
	wg.Wait()
}
