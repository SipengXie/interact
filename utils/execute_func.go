package utils

import (
	cachestate "interact/cacheState"

	statedb "github.com/ethereum/go-ethereum/core/state"
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
