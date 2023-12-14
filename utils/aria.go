package utils

import (
	"fmt"
	"interact/accesslist"
	"interact/core"
	interactState "interact/state"
	"interact/tracer"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/panjf2000/ants/v2"
)

func AriaOneRound(antsPool *ants.Pool, txs types.Transactions, header *types.Header,
	fakeChainCtx core.ChainContext, fullcache *interactState.FullCacheConcurrent, PrefetchRwSetList []accesslist.RWSetList,
	antsWG *sync.WaitGroup) (types.Transactions, accesslist.RWSetList) {

	fmt.Println("Start Aria One Round Execution")
	st := time.Now()
	cacheStates := GenerateCacheStatesConcurrent(antsPool, fullcache, PrefetchRwSetList, antsWG)
	snapshots := make([]*interactState.StateWithRwSets, len(txs))
	for i := 0; i < len(txs); i++ {
		snapshots[i] = interactState.NewStateWithRwSets(cacheStates[i])
	}
	readReserve := accesslist.NewReserveSet()
	writeReserve := accesslist.NewReserveSet()
	txListIndex := make([]int, len(txs))
	for i := range txListIndex {
		txListIndex[i] = i
	}

	errs := tracer.ExecWithSnapshotState(antsPool, txs, txListIndex, snapshots, header, fakeChainCtx, antsWG, readReserve, writeReserve)

	restTx := make(types.Transactions, 0)
	restPredictRwSets := make([]*accesslist.RWSet, 0)
	commitStates := make(interactState.CacheStateList, 0)

	for i, tx := range txs {
		if errs[i] != nil && errs[i] != tracer.ErrFalsePredict {
			// here must occur logic error
			// so we must deal it next time
			restTx = append(restTx, tx)
			restPredictRwSets = append(restPredictRwSets, snapshots[i].GetRWSet())

		} else if writeReserve.HasConflict(uint(i), snapshots[i].GetRWSet().WriteSet) {
			// WAW error
			restTx = append(restTx, tx)
			restPredictRwSets = append(restPredictRwSets, snapshots[i].GetRWSet())
		} else if readReserve.HasConflict(uint(i), snapshots[i].GetRWSet().WriteSet) && writeReserve.HasConflict(uint(i), snapshots[i].GetRWSet().ReadSet) {
			// Has both RAW and WAR error
			restTx = append(restTx, tx)
			restPredictRwSets = append(restPredictRwSets, snapshots[i].GetRWSet())
		} else {
			// can be committed
			commitStates = append(commitStates, snapshots[i].GetStateDB().(*interactState.CacheState))
		}
	}
	MergeToCacheStateConcurrent(antsPool, commitStates, fullcache, antsWG)
	fmt.Println("End Aria One Round Execution")
	fmt.Println("Cost:", time.Since(st))

	return restTx, restPredictRwSets
}
