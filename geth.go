package main

import (
	"fmt"
	"interact/accesslist"
	"interact/core"
	interactState "interact/state"
	"interact/tracer"
	"interact/utils"
	testfunc "interact/utils/testFunc"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/panjf2000/ants/v2"

	ethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

func ExecSerial(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) error {
	fmt.Println("SerialExecution")
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	txs, _, headers := utils.GetTxsPredictsAndHeaders(chainDB, sdbBackend, startNum, endNum)

	trueRWlists, _ := testfunc.TrueRWSets(txs[0], chainDB, sdbBackend, startNum)
	totalRWSet := make(accesslist.ALTuple)
	for _, rwset := range trueRWlists {
		for addr, state := range rwset.ReadSet {
			for hash := range state {
				totalRWSet.Add(addr, hash)
			}
		}
		for addr, state := range rwset.WriteSet {
			for hash := range state {
				totalRWSet.Add(addr, hash)
			}
		}
	}
	fmt.Println("Total Slots:", len(totalRWSet))
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

// Deprecated, only if we want to test for the non-concurrent prefetch time
func ExecWithConnectedComponents(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) error {
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

// Deprecated, only if we want to test for the non-concurrent prefetch time
func ExecWithDegreeZero(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) error {
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
			fullcache := interactState.NewCacheState()
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
				cacheStates := make(interactState.CacheStateList, len(groups[round]))

				for index := 0; index < len(groups[round]); index++ {
					// only prefectch for one transaction
					// groups[round][index] is the very tx we want to add to pool
					txid := groups[round][index]
					tx := txs[i][txid]
					cacheForOneTx := interactState.NewCacheState()
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
				utils.MergeToState(cacheStates, fullcache)
				PureMergeCost += time.Since(mergest)
			}
			utils.MergeToState(interactState.CacheStateList{fullcache}, state)

			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExection Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}

	}

	return nil
}

// Deprecated, only if we want to test for the non-concurrent prefetch time
func ExecWithMIS(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) error {
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
			fullcache := interactState.NewCacheState()

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
				cacheStates := make(interactState.CacheStateList, len(groups[round]))

				for index := 0; index < len(groups[round]); index++ {
					// only prefectch for one transaction
					// groups[round][index] is the very tx we want to add to pool
					txid := groups[round][index]
					tx := txs[i][txid]
					cacheForOneTx := interactState.NewCacheState()
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
				utils.MergeToState(cacheStates, fullcache)
				PureMergeCost += time.Since(mergest)
			}
			utils.MergeToState(interactState.CacheStateList{fullcache}, state)
			fmt.Println("Execution Time:", time.Since(st))
			fmt.Println("PureExection Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}
	}
	return nil
}

// Deprecated, only if we want to test for the non-concurrent prefetch time
func ExecAriaMultiRoundWithConcurrentState(chainDB ethdb.Database, sdbBackend ethState.Database, startNum, endNum uint64) error {
	fmt.Println("Aria Method")
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

		// for i'th block
		for i := 0; i < len(txs); i++ {
			PureExecutionCost := time.Duration(0)
			PurePrefetchCost := time.Duration(0)
			PureMergeCost := time.Duration(0)

			txListIndex := make([]int, len(txs[i])) // global index for identify the tx, predictList and trueList
			for j := 0; j < len(txs[i]); j++ {
				txListIndex[j] = j
			}

			// make a fullcache containing both predictList and trueList
			trueRWlists, _ := testfunc.TrueRWSets(txs[i], chainDB, sdbBackend, startNum+uint64(i))
			fullcache := interactState.NewFullCacheConcurrent()
			fullcache.Prefetch(state, trueRWlists)
			fullcache.Prefetch(state, predictRWSets[i])

			for {
				fmt.Println("Execute Parallel:", len(txListIndex))
				rwSetList := make([]accesslist.RWSetList, len(txListIndex))
				for j, index := range txListIndex {
					rwSetList[j] = accesslist.RWSetList{predictRWSets[i][index], trueRWlists[index]}
				}
				prefetchSt := time.Now()
				cacheStates := utils.GenerateCacheStatesConcurrent(antsPool, fullcache, rwSetList, &antsWG)
				PurePrefetchCost = PurePrefetchCost + time.Since(prefetchSt)

				snapshots := make([]*interactState.StateWithRwSets, len(txListIndex))
				for j := 0; j < len(txListIndex); j++ {
					snapshots[j] = interactState.NewStateWithRwSets(cacheStates[j])
				}

				readReserve := accesslist.NewReserveSet()
				writeReserve := accesslist.NewReserveSet()

				execSt := time.Now()
				errs := tracer.ExecWithSnapshotState(antsPool, txs[i], txListIndex, snapshots, headers[i], fakeChainCtx, &antsWG, readReserve, writeReserve)
				PureExecutionCost = PureExecutionCost + time.Since(execSt)
				fmt.Println("Exec Time:", time.Since(execSt))

				canCommit := make([]int, 0)       // contains j in local
				nextTxlistIndex := make([]int, 0) // contains index in global

				for j, index := range txListIndex {
					if errs[j] != nil {
						fmt.Println(errs[j])
					}
					if writeReserve.HasConflict(uint(index), snapshots[j].GetRWSet().WriteSet) { // WAW
						nextTxlistIndex = append(nextTxlistIndex, index)
						continue
					}
					if !readReserve.HasConflict(uint(index), snapshots[j].GetRWSet().WriteSet) || !writeReserve.HasConflict(uint(index), snapshots[j].GetRWSet().ReadSet) {
						if errs[j] == nil {
							canCommit = append(canCommit, j) // specify which snapshot can be merged into fullcache
						} else {
							nextTxlistIndex = append(nextTxlistIndex, index)
						}
					}
				}
				commitCacheStates := make(interactState.CacheStateList, len(canCommit))
				for k, j := range canCommit {
					commitCacheStates[k] = snapshots[j].GetStateDB().(*interactState.CacheState)
				}

				mergeSt := time.Now()
				utils.MergeToCacheStateConcurrent(antsPool, commitCacheStates, fullcache, &antsWG)
				PureMergeCost = PureMergeCost + time.Since(mergeSt)

				txListIndex = nextTxlistIndex
				if len(txListIndex) == 0 {
					break
				}
			}
			fmt.Println("PureExecution Time:", PureExecutionCost)
			fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
			fmt.Println("PureMergeInTurn Time:", PureMergeCost)
		}

	}

	return nil
}

func ExecWithConnectedComponentsConcurrentCacheState(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("Connected Components Solution Concurrent CacheState")
	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)

	var antsWG sync.WaitGroup
	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()

	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}

	start := time.Now()
	txGroupsList, RWSetGroupsList := utils.GenerateTxAndRWSetGroups(txs, predictRwSets)
	elapsed := time.Since(start)
	fmt.Println("Generate TxGroups Costs:", elapsed)

	// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!

	fullcache := interactState.NewFullCacheConcurrent()
	// here we don't pre warm the data
	fullcache.Prefetch(state, predictRwSets)

	st := time.Now()
	PureExecutionCost := time.Duration(0)
	PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
	PureMergeCost := time.Duration(0)

	startPrefetch := time.Now()
	cacheStates := utils.GenerateCacheStatesConcurrent(antsPool, fullcache, RWSetGroupsList, &antsWG)
	PurePrefetchCost = time.Since(startPrefetch)

	start = time.Now()
	tracer.ExecuteWithAntsPool(antsPool, txGroupsList, cacheStates, header, fakeChainCtx, &antsWG)
	PureExecutionCost = time.Since(start)

	startMerge := time.Now()
	utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
	PureMergeCost = time.Since(startMerge)

	fmt.Println("Execution Time:", time.Since(st))
	fmt.Println("PureExecution Time:", PureExecutionCost)
	fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
	fmt.Println("PureMergeInTurn Time:", PureMergeCost)

	return nil
}

func ExecWithDegreeZeroConcurrentCacheState(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("DegreeZero Solution Concurrent CacheState")

	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)

	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}

	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup

	// the i'th block
	st := time.Now()
	groups := utils.GenerateDegreeZeroGroups(txs, predictRwSets)
	fmt.Println("Generate TxGroups:", time.Since(st))
	fullcache := interactState.NewFullCacheConcurrent()
	// here we don't pre warm the data
	fullcache.Prefetch(state, predictRwSets)
	st = time.Now()
	PureExecutionCost := time.Duration(0)
	PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
	PureMergeCost := time.Duration(0)
	for round := 0; round < len(groups); round++ {
		// here we can add logic if len(groups[round]) if less than a threshold
		// fmt.Println("parallel exec and commit:", len(groups[round]))
		// Create groups to execute
		prefst := time.Now()
		txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], txs, predictRwSets, &antsWG)
		PurePrefetchCost += time.Since(prefst)

		execst := time.Now()
		tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, header, fakeChainCtx, &antsWG)
		PureExecutionCost += time.Since(execst)
		mergest := time.Now()
		utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
		PureMergeCost += time.Since(mergest)
	}
	// Ignore merge to state as we use fullcache to represent statedb
	// and another reason is that the Range of sync.Map is hard to use.
	// utils.MergeToState(interactState.CacheStateList{fullcache}, state)

	fmt.Println("Execution Time:", time.Since(st))
	fmt.Println("PureExecution Time:", PureExecutionCost)
	fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
	fmt.Println("PureMergeInTurn Time:", PureMergeCost)

	return nil
}

func ExecWithMISConcurrentCacheState(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("MIS Solution Concurrent CacheState")

	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)
	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}

	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup

	st := time.Now()
	groups := utils.GenerateMISGroups(txs, predictRwSets)
	fmt.Println("Generate TxGroups:", time.Since(st))
	fullcache := interactState.NewFullCacheConcurrent()
	// here we don't pre warm the data
	fullcache.Prefetch(state, predictRwSets)
	st = time.Now()
	PureExecutionCost := time.Duration(0)
	PurePrefetchCost := time.Duration(0) // without considering the very first fullcache prefetch
	PureMergeCost := time.Duration(0)
	for round := 0; round < len(groups); round++ {
		// here we can add logic if len(groups[round]) if less than a threshold

		// Create groups to execute
		// fmt.Println("parallel exec and commit:", len(groups[round]))
		prefst := time.Now()
		txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], txs, predictRwSets, &antsWG)
		PurePrefetchCost += time.Since(prefst)

		execst := time.Now()
		tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, header, fakeChainCtx, &antsWG)
		PureExecutionCost += time.Since(execst)
		// fmt.Println("exec time:", time.Since(execst))

		mergest := time.Now()
		utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
		PureMergeCost += time.Since(mergest)
	}
	// Ignore merge to state as we use fullcache to represent statedb
	// and another reason is that the Range of sync.Map is hard to use.
	// utils.MergeToState(interactState.CacheStateList{fullcache}, state)

	fmt.Println("Execution Time:", time.Since(st))
	fmt.Println("PureExecution Time:", PureExecutionCost)
	fmt.Println("PurePrefetchInTurn Time:", PurePrefetchCost)
	fmt.Println("PureMergeInTurn Time:", PureMergeCost)

	return nil
}

func ExecAriaThenConnectedComponentsWithOneBlock(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("Aria Method Then Connected Components With One Block")
	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup

	// execution environment
	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)
	trueRWlists, _ := testfunc.TrueRWSets(txs, chainDB, sdbBackend, height)
	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}
	fullcache := interactState.NewFullCacheConcurrent()
	fullcache.Prefetch(state, trueRWlists)
	fullcache.Prefetch(state, predictRwSets)

	// first we use Aria method to commit txs and get rw sets
	PrefetchRwSetList := make([]accesslist.RWSetList, len(txs))
	for i := range txs {
		PrefetchRwSetList[i] = accesslist.RWSetList{predictRwSets[i], trueRWlists[i]}
	}
	restTx, restPredictRwSets := utils.AriaOneRound(antsPool, txs, header, fakeChainCtx, fullcache, PrefetchRwSetList, &antsWG)

	// then we use connected components method to commit the rest txs
	fmt.Println("Start Connected Components Execution")
	st := time.Now()
	txGroup, RwSetGroup := utils.GenerateTxAndRWSetGroups(restTx, restPredictRwSets)
	cacheStates := utils.GenerateCacheStatesConcurrent(antsPool, fullcache, RwSetGroup, &antsWG)
	tracer.ExecuteWithAntsPool(antsPool, txGroup, cacheStates, header, fakeChainCtx, &antsWG)
	utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
	fmt.Println("End Connected Components Execution")
	fmt.Println("Cost:", time.Since(st))
	return nil
}

func ExecAriaThenDegreeZeroWithOneBlock(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("Aria Method Then Degree Zero With One Block")
	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup

	// execution environment
	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)
	trueRWlists, _ := testfunc.TrueRWSets(txs, chainDB, sdbBackend, height)
	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}
	fullcache := interactState.NewFullCacheConcurrent()
	fullcache.Prefetch(state, trueRWlists)
	fullcache.Prefetch(state, predictRwSets)

	// first we use Aria method to commit txs and get rw sets
	PrefetchRwSetList := make([]accesslist.RWSetList, len(txs))
	for i := range txs {
		PrefetchRwSetList[i] = accesslist.RWSetList{predictRwSets[i], trueRWlists[i]}
	}
	restTx, restPredictRwSets := utils.AriaOneRound(antsPool, txs, header, fakeChainCtx, fullcache, PrefetchRwSetList, &antsWG)

	// then we use Degree Zero method to commit the rest txs
	fmt.Println("Start Degree Zero Execution")
	st := time.Now()
	groups := utils.GenerateDegreeZeroGroups(restTx, restPredictRwSets)
	for round := 0; round < len(groups); round++ {
		txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], restTx, restPredictRwSets, &antsWG)
		tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, header, fakeChainCtx, &antsWG)
		utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
	}
	fmt.Println("End Degree Zero Execution")
	fmt.Println("Cost:", time.Since(st))
	return nil
}

func ExecAriaThenMISWithOneBlock(chainDB ethdb.Database, sdbBackend ethState.Database, height uint64) error {
	fmt.Println("Aria Method Then MIS With One Block")
	antsPool, _ := ants.NewPool(16, ants.WithPreAlloc(true))
	defer antsPool.Release()
	var antsWG sync.WaitGroup

	// execution environment
	txs, predictRwSets, header, fakeChainCtx := utils.GetTxsPredictsAndHeadersForOneBlock(chainDB, sdbBackend, height)
	trueRWlists, _ := testfunc.TrueRWSets(txs, chainDB, sdbBackend, height)
	state, err := utils.GetState(chainDB, sdbBackend, height-1)
	if err != nil {
		return err
	}
	fullcache := interactState.NewFullCacheConcurrent()
	fullcache.Prefetch(state, trueRWlists)
	fullcache.Prefetch(state, predictRwSets)

	// first we use Aria method to commit txs and get rw sets
	PrefetchRwSetList := make([]accesslist.RWSetList, len(txs))
	for i := range txs {
		PrefetchRwSetList[i] = accesslist.RWSetList{predictRwSets[i], trueRWlists[i]}
	}
	restTx, restPredictRwSets := utils.AriaOneRound(antsPool, txs, header, fakeChainCtx, fullcache, PrefetchRwSetList, &antsWG)

	// then we use MIS method to commit the rest txs
	fmt.Println("Start MIS Execution")
	st := time.Now()
	groups := utils.GenerateMISGroups(restTx, restPredictRwSets)
	for round := 0; round < len(groups); round++ {
		txsToExec, cacheStates := utils.GenerateTxsAndCacheStatesWithAnts(antsPool, fullcache, groups[round], restTx, restPredictRwSets, &antsWG)
		tracer.ExecuteWithAntsCacheStateRoundByRound(antsPool, txsToExec, cacheStates, header, fakeChainCtx, &antsWG)
		utils.MergeToCacheStateConcurrent(antsPool, cacheStates, fullcache, &antsWG)
	}
	fmt.Println("End MIS Execution")
	fmt.Println("Cost:", time.Since(st))
	return nil
}

func main() {
	Node, chainDB, sdbBackend := utils.GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)

	// just test one block
	ExecSerial(chainDB, sdbBackend, num, num)
	fmt.Println()
	ExecWithConnectedComponentsConcurrentCacheState(chainDB, sdbBackend, num)
	fmt.Println()
	ExecWithDegreeZeroConcurrentCacheState(chainDB, sdbBackend, num)
	fmt.Println()
	ExecWithMISConcurrentCacheState(chainDB, sdbBackend, num)
	fmt.Println()
	ExecAriaThenConnectedComponentsWithOneBlock(chainDB, sdbBackend, num)
	fmt.Println()
	ExecAriaThenDegreeZeroWithOneBlock(chainDB, sdbBackend, num)
	fmt.Println()
	ExecAriaThenMISWithOneBlock(chainDB, sdbBackend, num)
}
