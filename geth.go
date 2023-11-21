package main

import (
	"fmt"
	"interact/accesslist"
	cachestate "interact/cacheState"
	"interact/core"
	"interact/tracer"
	"interact/utils"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/panjf2000/ants/v2"

	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

func ExecFromStartToEndSerial(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("DegreeZero Solution Concurrent CacheState")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, _, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

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
	return nil
}

func ExecFromStartToEndWithConnectedComponents(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("Connected Components Solution")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	// Parallel Execution
	{
		// timeCost := time.Duration(0)
		// pool := gopool.NewGoPool(16, gopool.WithResultCallback(func(result interface{}) {
		// 	if result.(time.Duration) > timeCost {
		// 		timeCost = result.(time.Duration)
		// 	}
		// }))
		// defer pool.Release()

		var antsWG sync.WaitGroup
		antsPool, _ := ants.NewPoolWithFunc(16, func(params interface{}) {
			defer antsWG.Done()
			ExecParams := params.(*tracer.ParameterForTxGroup)
			tracer.ExecuteTxs(ExecParams.CacheState, ExecParams.TxsGroup, ExecParams.Header, ExecParams.ChainCtx)
		}, ants.WithPreAlloc(true))
		defer antsPool.Release()

		// pondPool := pond.New(16, len(txs), pond.MinWorkers(10))

		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		start := time.Now()

		txGroupsList := make([][]types.Transactions, len(txs))
		RWSetGroupsList := make([][]accesslist.RWSetList, len(txs))
		for i := 0; i < len(txs); i++ {
			txGroupsList[i], RWSetGroupsList[i] = utils.GenerateTxAndRWSetGroups(txs[i], predictRWSets[i])
		}
		elapsed := time.Since(start)
		fmt.Println("Generate TxGroups Costs:", elapsed)

		// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!

		for i := 0; i < len(txs); i++ {
			utils.GenerateCacheStates(state, RWSetGroupsList[i]) // trick!!!!!!!!!!!

			st := time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)

			startPrefetch := time.Now()
			cacheStates := utils.GenerateCacheStates(state, RWSetGroupsList[i]) // This step is to warm up the cache
			PurePrefetchCost = time.Since(startPrefetch)
			// fmt.Println("Prefetching Costs:", elapsedPrefetch)

			// use gopool
			// tracer.ExecuteWithGopoolCacheState(pool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			// use ants pool
			start = time.Now()
			antsWG.Add(len(txGroupsList[i]))
			tracer.ExecuteWithAntsCacheState(antsPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx, &antsWG)
			PureExecutionCost = time.Since(start)
			// use pond pool
			// tracer.ExecuteWithPondCacheState(pondPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			// fmt.Println("Longest Task Costs for Execution Costs:", timeCost)
			// timeCost = time.Duration(0)

			startMerge := time.Now()
			utils.MergeToState(cacheStates, state)
			PureMergeCost = time.Since(startMerge)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExecution Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}
	}

	return nil
}

func ExecFromStartToEndWithDegreeZero(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("DegreeZero Solution")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	{
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()
		var antsWG sync.WaitGroup

		for i := 0; i < len(txs); i++ {
			// the i'th block
			st := time.Now()
			groups := utils.GenerateDegreeZeroGroups(txs[i], predictRWSets[i])
			fmt.Println("Generate TxGroups:", time.Since(st))
			fullcache := cachestate.NewStateDB()
			// here we don't pre warm the data
			fullcache.Prefetch(state, predictRWSets[i])
			st = time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)
			for round := 0; round < len(groups); round++ {
				// here we can add logic if len(groups[round]) if less than a threshold

				// Create groups to execute
				txsToExec := make(types.Transactions, len(groups[round]))
				cacheStates := make(cachestate.CacheStateList, len(groups[round]))
				antsWG.Add(len(groups[round]))

				for index := 0; index < len(groups[round]); index++ {
					// only prefectch for one transaction
					// groups[round][index] is the very tx we want to add to pool
					txid := groups[round][index]
					tx := txs[i][txid]
					cacheForOneTx := cachestate.NewStateDB()
					prefst := time.Now()
					cacheForOneTx.Prefetch(fullcache, accesslist.RWSetList{predictRWSets[i][txid]})
					PurePrefetchCost += time.Since(prefst)

					txsToExec[index] = tx
					cacheStates[index] = cacheForOneTx
				}
				execst := time.Now()
				tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, headers[i], fakeChainCtx, &antsWG)
				PureExecutionCost += time.Since(execst)

				mergest := time.Now()
				utils.MergeToCacheState(cacheStates, fullcache)
				PureMergeCost += time.Since(mergest)
			}
			utils.MergeToState(cachestate.CacheStateList{fullcache}, state)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExection Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}

	}

	return nil
}

func ExecFromStartToEndWithMIS(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("MIS Solution")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	{
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()
		var antsWG sync.WaitGroup

		for i := 0; i < len(txs); i++ {
			// the i'th block
			st := time.Now()
			groups := utils.GenerateMISGroups(txs[i], predictRWSets[i])
			fmt.Println("Generate TxGroups:", time.Since(st))
			fullcache := cachestate.NewStateDB()

			// here we don't pre warm the data
			fullcache.Prefetch(state, predictRWSets[i])
			st = time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)
			for round := 0; round < len(groups); round++ {
				// here we can add logic if len(groups[round]) if less than a threshold

				// Create groups to execute
				txsToExec := make(types.Transactions, len(groups[round]))
				cacheStates := make(cachestate.CacheStateList, len(groups[round]))
				antsWG.Add(len(groups[round]))

				for index := 0; index < len(groups[round]); index++ {
					// only prefectch for one transaction
					// groups[round][index] is the very tx we want to add to pool
					txid := groups[round][index]
					tx := txs[i][txid]
					cacheForOneTx := cachestate.NewStateDB()
					prefst := time.Now()
					cacheForOneTx.Prefetch(fullcache, accesslist.RWSetList{predictRWSets[i][txid]})
					PurePrefetchCost += time.Since(prefst)

					txsToExec[index] = tx
					cacheStates[index] = cacheForOneTx
				}
				execst := time.Now()
				tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, headers[i], fakeChainCtx, &antsWG)
				PureExecutionCost += time.Since(execst)

				mergest := time.Now()
				utils.MergeToCacheState(cacheStates, fullcache)
				PureMergeCost += time.Since(mergest)
			}
			utils.MergeToState(cachestate.CacheStateList{fullcache}, state)
			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExection Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}
	}
	return nil
}

func ExecFromStartToEndWithConnectedComponentsConcurrentCacheState(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("Connected Components Solution Concurrent CacheState")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	// Parallel Execution
	{
		// timeCost := time.Duration(0)
		// pool := gopool.NewGoPool(16, gopool.WithResultCallback(func(result interface{}) {
		// 	if result.(time.Duration) > timeCost {
		// 		timeCost = result.(time.Duration)
		// 	}
		// }))
		// defer pool.Release()

		var antsWG sync.WaitGroup
		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()

		// pondPool := pond.New(16, len(txs), pond.MinWorkers(10))

		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		start := time.Now()

		txGroupsList := make([][]types.Transactions, len(txs))
		RWSetGroupsList := make([][]accesslist.RWSetList, len(txs))
		for i := 0; i < len(txs); i++ {
			txGroupsList[i], RWSetGroupsList[i] = utils.GenerateTxAndRWSetGroups(txs[i], predictRWSets[i])
		}
		elapsed := time.Since(start)
		fmt.Println("Generate TxGroups Costs:", elapsed)

		// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!

		for i := 0; i < len(txs); i++ {
			fullcache := cachestate.NewFullCacheConcurrent()
			// here we don't pre warm the data
			fullcache.Prefetch(state, predictRWSets[i])

			st := time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)

			startPrefetch := time.Now()
			antsWG.Add(len(RWSetGroupsList[i]))
			cacheStates := utils.GenerateCacheStatesConcurrent(antsPool, fullcache, RWSetGroupsList[i], &antsWG)
			PurePrefetchCost = time.Since(startPrefetch)

			// fmt.Println("Prefetching Costs:", elapsedPrefetch)

			// use gopool
			// tracer.ExecuteWithGopoolCacheState(pool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			// use ants pool
			start = time.Now()
			antsWG.Add(len(txGroupsList[i]))
			tracer.ExecuteWithAntsPool(antsPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx, &antsWG)
			PureExecutionCost = time.Since(start)

			// use pond pool
			// tracer.ExecuteWithPondCacheState(pondPool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)

			startMerge := time.Now()
			antsWG.Add(len(cacheStates))
			utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
			PureMergeCost = time.Since(startMerge)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExecution Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}
	}

	return nil
}

func ExecFromStartToEndWithDegreeZeroConcurrentCacheState(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("DegreeZero Solution Concurrent CacheState")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	{
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()
		var antsWG sync.WaitGroup

		for i := 0; i < len(txs); i++ {
			// the i'th block
			st := time.Now()
			groups := utils.GenerateDegreeZeroGroups(txs[i], predictRWSets[i])
			fmt.Println("Generate TxGroups:", time.Since(st))
			fullcache := cachestate.NewFullCacheConcurrent()
			// here we don't pre warm the data
			fullcache.Prefetch(state, predictRWSets[i])
			st = time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)
			for round := 0; round < len(groups); round++ {
				// here we can add logic if len(groups[round]) if less than a threshold

				// Create groups to execute
				antsWG.Add(len(groups[round]))
				prefst := time.Now()
				txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], txs[i], predictRWSets[i], &antsWG)
				PurePrefetchCost += time.Since(prefst)

				antsWG.Add(len(groups[round]))
				execst := time.Now()
				tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, headers[i], fakeChainCtx, &antsWG)
				PureExecutionCost += time.Since(execst)

				antsWG.Add(len(groups[round]))
				mergest := time.Now()
				utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
				PureMergeCost += time.Since(mergest)
			}
			// Ignore merge to state as we use fullcache to represent statedb
			// and another reason is that the Range of sync.Map is hard to use.
			// utils.MergeToState(cachestate.CacheStateList{fullcache}, state)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExecution Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}

	}

	return nil
}

func ExecFromStartToEndWithMISConcurrentCacheState(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	fmt.Println("MIS Solution Concurrent CacheState")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, predictRWSets, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

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
		state, err := utils.GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
		defer antsPool.Release()
		var antsWG sync.WaitGroup

		for i := 0; i < len(txs); i++ {
			// the i'th block
			st := time.Now()
			groups := utils.GenerateMISGroups(txs[i], predictRWSets[i])
			fmt.Println("Generate TxGroups:", time.Since(st))
			fullcache := cachestate.NewFullCacheConcurrent()
			// here we don't pre warm the data
			fullcache.Prefetch(state, predictRWSets[i])
			st = time.Now()
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
			PureMergeCost := time.Duration(0)
			for round := 0; round < len(groups); round++ {
				// here we can add logic if len(groups[round]) if less than a threshold

				// Create groups to execute
				antsWG.Add(len(groups[round]))
				prefst := time.Now()
				txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], txs[i], predictRWSets[i], &antsWG)
				PurePrefetchCost += time.Since(prefst)

				antsWG.Add(len(groups[round]))
				execst := time.Now()
				tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, headers[i], fakeChainCtx, &antsWG)
				PureExecutionCost += time.Since(execst)

				antsWG.Add(len(groups[round]))
				mergest := time.Now()
				utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
				PureMergeCost += time.Since(mergest)
			}
			// Ignore merge to state as we use fullcache to represent statedb
			// and another reason is that the Range of sync.Map is hard to use.
			// utils.MergeToState(cachestate.CacheStateList{fullcache}, state)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExecution Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}

	}

	return nil
}

func main() {
	Node, chainDB, sdbBackend := utils.GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)

	// just test one block
	ExecFromStartToEndSerial(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithConnectedComponents(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithDegreeZero(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithMIS(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithConnectedComponentsConcurrentCacheState(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithDegreeZeroConcurrentCacheState(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecFromStartToEndWithMISConcurrentCacheState(chainDB, sdbBackend, num, num)
}
