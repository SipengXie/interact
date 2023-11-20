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
