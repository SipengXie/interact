package main

import (
	"fmt"
	"interact/accesslist"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	"interact/fullstate"
	"interact/tracer"
	"interact/utils"
	"sync"
	"time"

	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/panjf2000/ants/v2"

	// "github.com/ethereum/go-ethereum/core/state"
	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

// two manners to execute the transactions test
func ExeTestFromStartToEndWithCacheState(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	// Try to expand transaction lists
	txs := make([]types.Transactions, endNum-startNum+1)
	predictRWSets := make([][]*accesslist.RWSet, endNum-startNum+1)
	headers := make([]*types.Header, endNum-startNum+1)
	TotalTxsNum := 0

	// generate predictlist, txslist and headerlist
	for height := startNum; height <= endNum; height++ {
		// for each block, we predict its txs
		block, header := utils.GetBlockAndHeader(chainDB, height)
		blockTxs := block.Transactions()
		blockPredicts := make([]*accesslist.RWSet, blockTxs.Len())
		for j, tx := range blockTxs {
			blockPredicts[j] = utils.PredictRWSets(tx, chainDB, sdbBackend, height)
		}
		txs[height-startNum] = blockTxs
		predictRWSets[height-startNum] = blockPredicts
		headers[height-startNum] = header
		TotalTxsNum += blockTxs.Len()
	}

	fmt.Println("Transaction Number:", TotalTxsNum)
	// predict the rw access list

	{
		// get statedb from block[startNum-1].Root
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}
		state.StartPrefetcher("miner")
		defer state.StopPrefetcher()

		// set the satrt time
		start := time.Now()
		// test the serial execution
		for i := 0; i < len(txs); i++ {
			tracer.ExecuteTxs(state, txs[i], headers[i], fakeChainCtx)
		}
		// cal the execution time
		elapsed := time.Since(start)
		fmt.Println("Serial Execution Time:", elapsed)
	}

	{
		timeCost := time.Duration(0)
		pool := gopool.NewGoPool(16, gopool.WithResultCallback(func(result interface{}) {
			if result.(time.Duration) > timeCost {
				timeCost = result.(time.Duration)
			}
		}))
		defer pool.Release()

		// // get statedb from block[startNum-1].Root
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}
		// state.StartPrefetcher("miner")
		// defer state.StopPrefetcher()

		start := time.Now()
		vertexGroupsList := make([][][]*conflictgraph.Vertex, len(txs))
		txGroupsList := make([][]types.Transactions, len(txs))
		RWSetGroupsList := make([][]accesslist.RWSetList, len(txs))
		for i := 0; i < len(txs); i++ {
			vertexGroupsList[i] = utils.GenerateVertexGroups(txs[i], predictRWSets[i])
			txGroupsList[i], RWSetGroupsList[i] = utils.GenerateTxAndRWSetGroups(vertexGroupsList[i], txs[i], predictRWSets[i])
		}
		elapsed := time.Since(start)
		fmt.Println("Generate TxGroups Costs:", elapsed)
		fmt.Println()

		// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!
		executeCost := time.Duration(0)
		for i := 0; i < len(txs); i++ {
			// // this step simulate that the state is committed
			// state, _ := GetState(chainDB, sdbBackend, startNum-1+uint64(i))
			// start = time.Now()
			// cacheStates := GenerateCacheStatesWithPool(pool, state, RWSetGroupsList[i])
			// elapsed = time.Since(start)
			// executeCost += elapsed

			// fmt.Println("Longest Task for Prefecthing Costs:", timeCost)
			// timeCost = time.Duration(0)
			startPrefetch := time.Now()
			cacheStates := utils.GenerateCacheStates(state, RWSetGroupsList[i]) // This step is to warm up the cache
			elapsedPrefetch := time.Since(startPrefetch)
			fmt.Println("Prefetching Costs:", elapsedPrefetch)

			start = time.Now()
			tracer.ExecuteWithGopoolCacheState(pool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)
			elapsed = time.Since(start)
			executeCost += elapsed

			fmt.Println("Longest Task Costs for Execution Costs:", timeCost)
			timeCost = time.Duration(0)
			utils.MergeToState(cacheStates, state)
			// fmt.Println()
		}
		fmt.Println("Parallel Execution Time With cacheState:", executeCost)
	}

	return nil
}

func ExeTestOneBlockWithCacheState(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	// Try to expand transaction lists
	txs := make([]types.Transactions, endNum-startNum+1)
	predictRWSets := make([][]*accesslist.RWSet, endNum-startNum+1)
	headers := make([]*types.Header, endNum-startNum+1)
	TotalTxsNum := 0

	// generate predictlist, txslist and headerlist
	for height := startNum; height <= endNum; height++ {
		// for each block, we predict its txs
		block, header := utils.GetBlockAndHeader(chainDB, height)
		blockTxs := block.Transactions()
		blockPredicts := make([]*accesslist.RWSet, blockTxs.Len())
		for j, tx := range blockTxs {
			blockPredicts[j] = utils.PredictRWSets(tx, chainDB, sdbBackend, height)
		}
		txs[height-startNum] = blockTxs
		predictRWSets[height-startNum] = blockPredicts
		headers[height-startNum] = header
		TotalTxsNum += blockTxs.Len()
	}

	fmt.Println("Transaction Number:", TotalTxsNum)
	// predict the rw access list

	// Serial Execution
	{
		// get statedb from block[startNum-1].Root
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		// set the satrt time
		start := time.Now()
		// test the serial execution
		for i := 0; i < len(txs); i++ {
			tracer.ExecuteTxs(state, txs[i], headers[i], fakeChainCtx)
		}
		// cal the execution time
		elapsed := time.Since(start)
		fmt.Println("Serial Execution Time:", elapsed)
	}

	// Parallel Execution
	{
		timeCost := time.Duration(0)
		pool := gopool.NewGoPool(16, gopool.WithResultCallback(func(result interface{}) {
			if result.(time.Duration) > timeCost {
				timeCost = result.(time.Duration)
			}
		}))
		defer pool.Release()

		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()
		var antsWG sync.WaitGroup

		// pondPool := pond.New(16, len(txs), pond.MinWorkers(10))

		// use the same state to predict
		// 1 get the block and txs
		block, header := utils.GetBlockAndHeader(chainDB, startNum)
		blockTransactions := block.Transactions()

		// 2 New a statedb(get statedb from block[startNum-1].Root)
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}
		// new the fullstate
		fulldb := fullstate.NewFullState(state.Copy()) // use copy
		// fulldb := fullstate.NewFullState(state) // no use the copy

		// 3 Predict the rw access list
		blockPredicts := make([]*accesslist.RWSet, blockTransactions.Len())
		for j, tx := range blockTransactions {
			blockPredicts[j], err = tracer.ExecToGenerateRWSet(fulldb, tx, header, fakeChainCtx)
			if err != nil {
				fmt.Println("NIL tx hash:", tx.Hash())
			}
		}

		start := time.Now()
		vertexGroupsList := make([][][]*conflictgraph.Vertex, len(txs))
		txGroupsList := make([][]types.Transactions, len(txs))
		RWSetGroupsList := make([][]accesslist.RWSetList, len(txs))
		for i := 0; i < len(txs); i++ {
			vertexGroupsList[i] = utils.GenerateVertexGroups(txs[i], predictRWSets[i])
			txGroupsList[i], RWSetGroupsList[i] = utils.GenerateTxAndRWSetGroups(vertexGroupsList[i], txs[i], predictRWSets[i])
		}
		elapsed := time.Since(start)
		fmt.Println("Generate TxGroups Costs:", elapsed)
		fmt.Println()

		// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!
		executeCost := time.Duration(0)
		for i := 0; i < len(txs); i++ {
			utils.GenerateCacheStates(state, RWSetGroupsList[i]) // trick!!!!!!!!!!!

			startPrefetch := time.Now()
			cacheStates := utils.GenerateCacheStates(state, RWSetGroupsList[i]) // This step is to warm up the cache
			elapsedPrefetch := time.Since(startPrefetch)
			fmt.Println("Prefetching Costs:", elapsedPrefetch)

			start = time.Now()
			// use gopool
			// tracer.ExecuteWithGopoolCacheState(pool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			// use ants pool
			antsWG.Add(len(txGroupsList[i]))
			tracer.ExecuteWithAntsCacheState(antsPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx, &antsWG)

			// use pond pool
			// tracer.ExecuteWithPondCacheState(pondPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			elapsed = time.Since(start)
			executeCost += elapsed

			fmt.Println("Longest Task Costs for Execution Costs:", timeCost)
			timeCost = time.Duration(0)

			startMerge := time.Now()
			utils.MergeToState(cacheStates, state)
			elapsedMerge := time.Since(startMerge)
			fmt.Println("Merge Costs:", elapsedMerge)
		}

		fmt.Println("Parallel Execution Time With cacheState:", executeCost)
	}

	return nil
}

func main() {
	Node, chainDB, sdbBackend := utils.GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)
	// IterateBlock(chainDB, sdbBackend, num)
	// CompareTracerAndFulldb(chainDB, sdbBackend, num)
	// ExeTestFromStartToEndWithCacheState(chainDB, sdbBackend, num-19, num)
	// ExeTestFromStartToEndWithStateDB(chainDB, sdbBackend, num, num)

	// just test one block
	ExeTestOneBlockWithCacheState(chainDB, sdbBackend, num, num)
}
