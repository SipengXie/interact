package utils

import (
	"fmt"
	"interact/accesslist"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	interactState "interact/state"
	"interact/tracer"
	"sort"
	"sync"

	"github.com/ethereum/go-ethereum/core/rawdb"
	ethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/panjf2000/ants/v2"
)

// PredictRWSets predict a tx rwsets in a block with accesslist
func PredictRWSets(tx *types.Transaction, chainDB ethdb.Database, sdbBackend ethState.Database, num uint64) *accesslist.RWSet {

	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := ethState.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}
	fulldb := interactState.NewStateWithRwSets(state)

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	list, err := tracer.ExecToGenerateRWSet(fulldb, tx, header, fakeChainCtx)
	if err != nil {
		fmt.Println("NIL tx hash:", tx.Hash())
	}
	return list
}

func generateUndiGraph(txs types.Transactions, predictRWSets []*accesslist.RWSet) *conflictgraph.UndirectedGraph {
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
	return undiConfGraph
}

func generateVertexGroups(txs types.Transactions, predictRWSets []*accesslist.RWSet) [][]*conflictgraph.Vertex {
	undiConfGraph := generateUndiGraph(txs, predictRWSets)
	groups := undiConfGraph.GetConnectedComponents()
	return groups
}

func GetTxsPredictsAndHeadersForOneBlock(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) (types.Transactions, accesslist.RWSetList, *types.Header, core.ChainContext) {
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	block, header := GetBlockAndHeader(chainDB, height)
	txs := block.Transactions()

	// predict and true used to fetch data from statedb
	// to construct a state for testing
	predictRwSets := make([]*accesslist.RWSet, txs.Len())
	for i, tx := range txs {
		predictRwSets[i] = PredictRWSets(tx, chainDB, sdbBackend, height)
	}
	return txs, predictRwSets, header, fakeChainCtx
}

func GetTxsPredictsAndHeaders(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) ([]types.Transactions, []accesslist.RWSetList, []*types.Header) {
	// Try to expand transaction lists
	txs := make([]types.Transactions, endNum-startNum+1)
	predictRWSets := make([]accesslist.RWSetList, endNum-startNum+1)
	headers := make([]*types.Header, endNum-startNum+1)
	TotalTxsNum := 0

	// generate predictlist, txslist and headerlist
	for height := startNum; height <= endNum; height++ {
		// for each block, we predict its txs
		block, header := GetBlockAndHeader(chainDB, height)
		blockTxs := block.Transactions()
		blockPredicts := make([]*accesslist.RWSet, blockTxs.Len())
		for j, tx := range blockTxs {
			blockPredicts[j] = PredictRWSets(tx, chainDB, sdbBackend, height)
		}
		txs[height-startNum] = blockTxs
		predictRWSets[height-startNum] = blockPredicts
		headers[height-startNum] = header
		TotalTxsNum += blockTxs.Len()
	}

	fmt.Println("Transaction Number:", TotalTxsNum)

	return txs, predictRWSets, headers
}

func GenerateTxAndRWSetGroups(txs types.Transactions, predictRWSets accesslist.RWSetList) ([]types.Transactions, []accesslist.RWSetList) {
	vertexGroup := generateVertexGroups(txs, predictRWSets)
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

func GenerateMISGroups(txs types.Transactions, predictRWSets accesslist.RWSetList) [][]uint {
	undiGraph := generateUndiGraph(txs, predictRWSets)
	return solveMISInTurn(undiGraph)
}

func generateDiGraph(txs types.Transactions, predictRWSets []*accesslist.RWSet) *conflictgraph.DirectedGraph {
	Graph := conflictgraph.NewDirectedGraph()
	for i, tx := range txs {
		if predictRWSets[i] == nil {
			continue
		}
		Graph.AddVertex(tx.Hash(), uint(i))
	}
	for i := 0; i < txs.Len(); i++ {
		for j := i + 1; j < txs.Len(); j++ {
			if predictRWSets[i] == nil || predictRWSets[j] == nil {
				continue
			}
			if predictRWSets[i].HasConflict(*predictRWSets[j]) {
				Graph.AddEdge(uint(i), uint(j))
			}
		}
	}
	return Graph
}

func GenerateDegreeZeroGroups(txs types.Transactions, predictRWSets []*accesslist.RWSet) [][]uint {
	graph := generateDiGraph(txs, predictRWSets)
	return graph.GetDegreeZero()
}

func GenerateCacheStates(db vm.StateDB, RWSetsGroups []accesslist.RWSetList) interactState.CacheStateList {
	// cannot concurrent prefetch due to the statedb is not thread safe
	cacheStates := make([]*interactState.CacheState, len(RWSetsGroups))
	for i := 0; i < len(RWSetsGroups); i++ {
		if RWSetsGroups[i] == nil {
			continue
		}
		cacheStates[i] = interactState.NewCacheState()
		cacheStates[i].Prefetch(db, RWSetsGroups[i])
	}
	return cacheStates
}

func GenerateCacheStatesConcurrent(pool *ants.Pool, db vm.StateDB, RWSetsGroups []accesslist.RWSetList, wg *sync.WaitGroup) interactState.CacheStateList {
	cacheStates := make([]*interactState.CacheState, len(RWSetsGroups))
	wg.Add(len(RWSetsGroups))
	for i := 0; i < len(RWSetsGroups); i++ {
		if RWSetsGroups[i] == nil {
			wg.Done()
			continue
		}
		index := i
		err := pool.Submit(func() {
			cacheStates[index] = interactState.NewCacheState()
			cacheStates[index].Prefetch(db, RWSetsGroups[index])
			wg.Done()
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done()
		}
	}
	wg.Wait()
	return cacheStates
}

func GenerateTxToExec(group []uint, txs types.Transactions) types.Transactions {
	txsToExec := make(types.Transactions, len(group))
	for i := 0; i < len(group); i++ {
		txsToExec[i] = txs[group[i]]
	}
	return txsToExec
}

func GenerateTxsAndCacheStatesWithAnts(pool *ants.Pool, db *interactState.FullCacheConcurrent, group []uint, txs types.Transactions, predictList accesslist.RWSetList, wg *sync.WaitGroup) (types.Transactions, interactState.CacheStateList) {
	txsToExec := make(types.Transactions, len(group))
	cacheStates := make([]*interactState.CacheState, len(group))
	wg.Add(len(group))
	for i := 0; i < len(group); i++ {
		index := i
		txid := group[index]
		tx := txs[txid]
		txsToExec[index] = tx

		err := pool.Submit(func() {
			cacheStates[index] = interactState.NewCacheState()
			cacheStates[index].Prefetch(db, accesslist.RWSetList{predictList[txid]})
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}

	}
	wg.Wait()
	return txsToExec, cacheStates
}
