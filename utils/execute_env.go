package utils

import (
	"fmt"
	"interact/accesslist"
	cachestate "interact/cacheState"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	"interact/fullstate"
	"interact/tracer"
	"sort"
	"time"

	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/rawdb"
	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
)

// PredictRWSets predict a tx rwsets in a block with accesslist
func PredictRWSets(tx *types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) *accesslist.RWSet {

	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}
	fulldb := fullstate.NewFullState(state)

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	list, err := tracer.ExecToGenerateRWSet(fulldb, tx, header, fakeChainCtx)
	if err != nil {
		fmt.Println("NIL tx hash:", tx.Hash())
	}
	return list
}

func GenerateTxAndRWSetGroups(vertexGroup [][]*conflictgraph.Vertex, txs types.Transactions, predictRWSets accesslist.RWSetList) ([]types.Transactions, []accesslist.RWSetList) {
	// From vertex group to transaction group
	txsGroup := make([]types.Transactions, len(vertexGroup))
	RWSetsGroup := make([]accesslist.RWSetList, len(vertexGroup))
	for i := 0; i < len(vertexGroup); i++ {
		sort.Slice(vertexGroup[i], func(j, k int) bool {
			return vertexGroup[i][j].TxId < vertexGroup[i][k].TxId
		})

		for j := 0; j < len(vertexGroup[i]); j++ {
			txsGroup[i] = append(txsGroup[i], txs[vertexGroup[i][j].TxId])
			RWSetsGroup[i] = append(RWSetsGroup[i], predictRWSets[vertexGroup[i][j].TxId])
		}
	}
	return txsGroup, RWSetsGroup
}

func GenerateVertexGroups(txs types.Transactions, predictRWSets []*accesslist.RWSet) [][]*conflictgraph.Vertex {
	undiConfGraph := conflictgraph.NewUndirectedGraph()
	for i, tx := range txs {
		if predictRWSets[i] == nil {
			continue
		}
		undiConfGraph.AddVertex(tx.Hash(), uint(i))
	}
	for i := 0; i < txs.Len(); i++ {
		for j := i + 1; j < txs.Len(); j++ {
			if predictRWSets[i] == nil || predictRWSets[j] == nil {
				continue
			}
			if predictRWSets[i].HasConflict(*predictRWSets[j]) {
				undiConfGraph.AddEdge(uint(i), uint(j))
			}
		}
	}

	groups := undiConfGraph.GetConnectedComponents()
	return groups
}

func GenerateCacheStates(db vm.StateDB, RWSetsGroups []accesslist.RWSetList) cachestate.CacheStateList {
	// cannot concurrent prefetch due to the statedb is not thread safe
	cacheStates := make([]*cachestate.CacheState, len(RWSetsGroups))
	for i := 0; i < len(RWSetsGroups); i++ {
		if RWSetsGroups[i] == nil {
			continue
		}
		cacheStates[i] = cachestate.NewStateDB()
		cacheStates[i].Prefetch(db, RWSetsGroups[i])
	}
	return cacheStates
}

// GenerateCacheStatesWithPool this function try to prefectch concurrently
// ! Cannot Continously run, for the hot data copy is...
func GenerateCacheStatesWithGopool(pool gopool.GoPool, db *statedb.StateDB, RWSetsGroups []accesslist.RWSetList) cachestate.CacheStateList {
	// cannot concurrent prefetch due to the statedb is not thread safe
	// st := time.Now()
	dbList := make([]*statedb.StateDB, len(RWSetsGroups))
	cacheStates := make([]*cachestate.CacheState, len(RWSetsGroups))
	for i := 0; i < len(RWSetsGroups); i++ {
		cacheStates[i] = cachestate.NewStateDB()
		dbList[i] = db.Copy()
	}

	for i := 0; i < len(RWSetsGroups); i++ {
		if RWSetsGroups[i] == nil {
			continue
		}
		taskNum := i
		pool.AddTask(func() (interface{}, error) {
			st := time.Now()
			cacheStates[taskNum].Prefetch(dbList[taskNum], RWSetsGroups[taskNum])
			return time.Since(st), nil
		})
	}
	pool.Wait()

	// fmt.Println("Concurrent Prefetching cost:", time.Since(st))
	return cacheStates
}
