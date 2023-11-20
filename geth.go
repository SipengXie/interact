package main

import (
	"fmt"
	"interact/accesslist"
	cachestate "interact/cacheState"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	"interact/fullstate"
	"interact/mis"
	"interact/tracer"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"

	// "github.com/ethereum/go-ethereum/core/state"
	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"
)

func GetEthDatabaseAndStateDatabase() (*node.Node, ethdb.Database, statedb.Database) {
	nodeCfg := node.Config{DataDir: "/mnt/disk1/xsp/chaindata/execution/"}
	Node, err := node.New(&nodeCfg)
	if err != nil {
		panic(err)
	}
	ethCfg := ethconfig.Defaults
	chainDB, err := Node.OpenDatabase("chaindata", ethCfg.DatabaseCache, ethCfg.DatabaseHandles, "eth/db/chaindata/", true)
	if err != nil {
		panic(err)
	}

	config := &trie.Config{Preimages: ethCfg.Preimages}
	config.PathDB = &pathdb.Config{
		StateHistory:   ethCfg.StateHistory,
		CleanCacheSize: 256 * 1024 * 1024,
		DirtyCacheSize: 256 * 1024 * 1024,
	}

	trieDB := trie.NewDatabase(chainDB, config)
	sdbBackend := statedb.NewDatabaseWithNodeDB(chainDB, trieDB)
	return Node, chainDB, sdbBackend
}

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

func TrueRWSets(txs []*types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) ([]*accesslist.RWSet, error) {
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		return nil, err
	}
	fulldb := fullstate.NewFullState(state)

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	lists, errs := tracer.CreateRWSetsWithTransactions(fulldb, txs, header, fakeChainCtx)
	for i, err := range errs {
		if err != nil {
			fmt.Println("In TRUERWSetsS, tx hash:", txs[i].Hash())
			panic(err)
		}
	}
	return lists, nil
}

func IterateBlock(chainDB ethdb.Database, sdbBackend statedb.Database, startHeight uint64) {
	num := startHeight
	file, _ := os.Create("test.txt")
	defer file.Close()
	for {
		fmt.Fprintln(file, "Processing Block Height:", num)
		headHash := rawdb.ReadCanonicalHash(chainDB, num)
		Block := rawdb.ReadBlock(chainDB, headHash, num)
		txs := Block.Transactions()

		predictLists := make([]*accesslist.RWSet, txs.Len())
		for i, tx := range txs {
			predictLists[i] = PredictRWSets(tx, chainDB, sdbBackend, num)
		}
		trueLists, err := TrueRWSets(txs, chainDB, sdbBackend, num)
		if err != nil {
			break
		}

		nilCounter := 0
		conflictCounter := 0
		for i, list := range trueLists {
			if predictLists[i] == nil {
				nilCounter++
				continue
			}
			if !list.Equal(*predictLists[i]) {
				conflictCounter++
			}
		}
		fmt.Fprintln(file, "Transaction Number", txs.Len())
		fmt.Fprintln(file, "Nil Prediction Number:", nilCounter)
		fmt.Fprintln(file, "False Prediction Number:", conflictCounter)

		undiConfGraph := conflictgraph.NewUndirectedGraph()
		for i, tx := range txs {
			undiConfGraph.AddVertex(tx.Hash(), uint(i))
		}

		for i := 0; i < txs.Len(); i++ {
			for j := i + 1; j < txs.Len(); j++ {
				if predictLists[i] == nil || predictLists[j] == nil {
					continue
				}
				if predictLists[i].HasConflict(*predictLists[j]) {
					undiConfGraph.AddEdge(uint(i), uint(j))
				}
			}
		}

		groups := undiConfGraph.GetConnectedComponents()
		fmt.Fprintln(file, "Number of Groups:", len(groups))
		for i := 0; i < len(groups); i++ {
			fmt.Fprintf(file, "Number of Group[%d]:%d\n", i, len(groups[i]))
		}
		num--
	}
}

func SolveMISInTurn(undiConfGraph *conflictgraph.UndirectedGraph) {
	for {
		MisSolution := mis.NewSolution(undiConfGraph)
		MisSolution.Solve()
		ansSlice := MisSolution.IndependentSet.ToSlice()
		fmt.Println(len(ansSlice))

		for _, v := range undiConfGraph.Vertices {
			v.IsDeleted = false
			v.Degree = uint(len(undiConfGraph.AdjacencyMap[v.TxId]))
		}
		if len(ansSlice) <= 3 {
			edgeCount := 0
			for id := range undiConfGraph.Vertices {
				edgeCount += len(undiConfGraph.AdjacencyMap[id])
			}
			edgeCount /= 2
			fmt.Println("Node Cound:", len(undiConfGraph.Vertices))
			fmt.Println("Edge Count:", edgeCount)
		}
		for _, v := range ansSlice {
			undiConfGraph.Vertices[v.(uint)].IsDeleted = true
		}
		undiConfGraph = undiConfGraph.CopyGraphWithDeletion()
		if len(undiConfGraph.Vertices) == 0 {
			break
		}
	}
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

// Try to prefectch concurrently
// ! Cannot Continously run, for the hot data copy is...
func GenerateCacheStatesWithPool(pool gopool.GoPool, db *statedb.StateDB, RWSetsGroups []accesslist.RWSetList) cachestate.CacheStateList {
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

func MergeToState(cacheStates cachestate.CacheStateList, db *statedb.StateDB) {
	for i := 0; i < len(cacheStates); i++ {
		cacheStates[i].MergeState(db)
	}
}

// Get StateDB from block[num].Root
func GetState(chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) (*statedb.StateDB, error) {
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num)
	return statedb.New(baseHeader.Root, sdbBackend, nil)
}

func GetBlockAndHeader(chainDB ethdb.Database, num uint64) (*types.Block, *types.Header) {
	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	block := rawdb.ReadBlock(chainDB, headHash, num)
	return block, header
}

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
	// predict the rw access list

	{
		// get statedb from block[startNum-1].Root
		state, err := GetState(chainDB, sdbBackend, startNum-1)
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
		state, err := GetState(chainDB, sdbBackend, startNum-1)
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
			vertexGroupsList[i] = GenerateVertexGroups(txs[i], predictRWSets[i])
			txGroupsList[i], RWSetGroupsList[i] = GenerateTxAndRWSetGroups(vertexGroupsList[i], txs[i], predictRWSets[i])
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

			cacheStates := GenerateCacheStates(state, RWSetGroupsList[i]) // This step is to warm up the cache

			start = time.Now()
			tracer.ExecuteWithGopoolCacheState(pool, txGroupsList[i], cacheStates, headers[i], fakeChainCtx)
			elapsed = time.Since(start)
			executeCost += elapsed

			fmt.Println("Longest Task Costs for Execution Costs:", timeCost)
			timeCost = time.Duration(0)
			MergeToState(cacheStates, state)
			// fmt.Println()
		}
		fmt.Println("Parallel Execution Time With cacheState:", executeCost)
	}

	return nil
}

// TODO: Try to achive with StateDB, we need statedb merge
func ExeTestFromStartToEndWithStateDB(chainDB ethdb.Database, sdbBackend statedb.Database, startNum, endNum uint64) error {
	if startNum != endNum {
		panic("startNum != endNum")
	}
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	// Try to expand transaction lists
	txs := make([]types.Transactions, endNum-startNum+1)
	predictRWSets := make([][]*accesslist.RWSet, endNum-startNum+1)
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
	// predict the rw access list

	{
		// get statedb from block[startNum-1].Root
		state, err := GetState(chainDB, sdbBackend, startNum-1)
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

		// get statedb from block[startNum-1].Root
		state, err := GetState(chainDB, sdbBackend, startNum-1)
		if err != nil {
			return err
		}

		start := time.Now()
		vertexGroupsList := make([][][]*conflictgraph.Vertex, len(txs))
		txGroupsList := make([][]types.Transactions, len(txs))
		RWSetGroupsList := make([][]accesslist.RWSetList, len(txs))
		for i := 0; i < len(txs); i++ {
			vertexGroupsList[i] = GenerateVertexGroups(txs[i], predictRWSets[i])
			txGroupsList[i], RWSetGroupsList[i] = GenerateTxAndRWSetGroups(vertexGroupsList[i], txs[i], predictRWSets[i])
		}
		elapsed := time.Since(start)
		fmt.Println("Generate TxGroups Costs:", elapsed)

		// !!! Our Prefetch is less efficient than StateDB.Prefetch !!!
		start = time.Now()
		for i := 0; i < len(txs); i++ {
			stateList := make([]*statedb.StateDB, len(RWSetGroupsList[i]))
			for j := 0; j < len(RWSetGroupsList[i]); j++ {
				stateList[j] = state.Copy()
			}
			tracer.ExecuteWithGopoolStateDB(pool, txGroupsList[i], stateList, headers[i], fakeChainCtx)
			fmt.Println("Longest Task Costs:", timeCost)
			// No Merge state yet
		}
		elapsed = time.Since(start)
		fmt.Println("Parallel Execution Time With cacheState:", elapsed)
	}

	return nil
}

func CompareTracerAndFulldb(chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) {
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	baseState, _ := GetState(chainDB, sdbBackend, num-1)
	block, header := GetBlockAndHeader(chainDB, num)

	rand.Seed(time.Now().UnixNano())
	randomId := rand.Intn(block.Transactions().Len())
	tx := block.Transactions()[randomId]
	fmt.Println("Tx id:", randomId)
	fmt.Println("Tx Hash:", tx.Hash().Hex())
	tracerPredict, _ := tracer.PredictWithTracer(baseState.Copy(), tx, header, fakeChainCtx)

	fulldb := fullstate.NewFullState(baseState.Copy())
	fullStatePredict, _ := tracer.ExecToGenerateRWSet(fulldb, tx, header, fakeChainCtx)

	jsonfile, _ := os.Create("tracer.json")
	fmt.Fprintln(jsonfile, tracerPredict.ToJsonStruct().ToString())
	jsonfile.Close()

	jsonfile, _ = os.Create("fullstate.json")
	fmt.Fprintln(jsonfile, fullStatePredict.ToJsonStruct().ToString())
	jsonfile.Close()
}

func main() {
	Node, chainDB, sdbBackend := GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)
	// IterateBlock(chainDB, sdbBackend, num)
	// CompareTracerAndFulldb(chainDB, sdbBackend, num)
	ExeTestFromStartToEndWithCacheState(chainDB, sdbBackend, num, num)
	// ExeTestFromStartToEndWithStateDB(chainDB, sdbBackend, num, num)
}
