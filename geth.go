package main

import (
	"fmt"
	"interact/accesslist"
	conflictgraph "interact/conflictGraph"
	"interact/mis"
	"interact/tracer"

	"github.com/ethereum/go-ethereum/core/rawdb"
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
	chainDB, err := Node.OpenDatabaseWithFreezer("chaindata", ethCfg.DatabaseCache, ethCfg.DatabaseHandles, ethCfg.DatabaseFreezer, "eth/db/chaindata/", true)
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

func PredictRWAL(tx *types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) *accesslist.RW_AccessLists {

	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	list, _ := tracer.CreateRWAL(state, tx, header)
	// listJSON := list.ToJSON()
	// b := common.Hex2Bytes(listJSON)
	// fmt.Println("Tx Hash is:", tx.Hash())
	// fmt.Println(string(b))

	return list
}

// PredictOldAL 获取预测的OldAccessList，即在不更新StateDB的条件下执行交易获取OldAccessList
func PredictOldAL(tx *types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) *accesslist.AccessList {

	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	list, _ := tracer.CreateOldAL(state, tx, header)

	return list
}

func TrueRWALs(txs []*types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) []*accesslist.RW_AccessLists {
	fmt.Println("Staring Run True RWALs")
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)

	lists := tracer.CreateRWALWithTransactions(state, txs, header)
	fmt.Println("Finishing Run True RWALs")
	return lists
}

func OldTrueALs(txs []*types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) []*accesslist.AccessList {
	fmt.Println("Staring Run True OldALs")
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)

	// ? 关键
	lists := tracer.CreateOldALWithTransactions(state, txs, header)

	fmt.Println("Finishing Run True RWALs")
	return lists
}

func main() {
	Node, chainDB, sdbBackend := GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)
	fmt.Println("Block Height:", num)
	headBlock := rawdb.ReadBlock(chainDB, head, num)
	txs := headBlock.Transactions()

	// TODO: 获取 真实的 NewAccessList
	trueLists := TrueRWALs(txs, chainDB, sdbBackend, num)

	// TODO: 获取 真实的 OldAccessList
	trueOldLists := OldTrueALs(txs, chainDB, sdbBackend, num)

	// Node.Close()

	// Node, chainDB, sdbBackend = GetEthDatabaseAndStateDatabase()
	predictLists := make([]*accesslist.RW_AccessLists, txs.Len())
	predictOldLists := make([]*accesslist.AccessList, txs.Len())
	fmt.Println("Staring Run Predicting RWALs")
	for i, tx := range txs {
		fmt.Printf("Starting Predicting Tx[%d]\n", i)
		predictLists[i] = PredictRWAL(tx, chainDB, sdbBackend, num)

		// TODO: 获取 预测的 OldAccessList
		predictOldLists[i] = PredictOldAL(tx, chainDB, sdbBackend, num)
	}
	fmt.Println("Finishing Run Predicting RWALs and OldALs")

	// 新AccessList两者冲突检测
	conflictCounter := 0
	nilCounter := 0
	conflictTxs := make([]int, 0)
	for i, list := range trueLists {
		if predictLists[i] == nil {
			nilCounter++
			continue
		}
		if !list.Equal(*predictLists[i]) {
			conflictCounter++
			conflictTxs = append(conflictTxs, i)
		}
	}

	// TODO:预测冲突率、实际冲突率实现
	// 旧AccessList两者冲突检测
	conflictCounter1 := 0
	nilCounter1 := 0
	conflictTxs1 := make([]int, 0)
	for i, list := range trueOldLists {
		if predictOldLists[i] == nil {
			nilCounter1++
			continue
		}
		if !list.OldEqual(*predictOldLists[i]) {
			conflictCounter1++
			conflictTxs1 = append(conflictTxs1, i)
		}
	}

	fmt.Println("Nil Prediction Number:", nilCounter)
	fmt.Println("False Prediction Number:", conflictCounter)

	fmt.Println("Old Nil Prediction Number:", nilCounter1)
	fmt.Println("Old False Prediction Number:", conflictCounter1)

	// 新的AccessList建图
	undiConfGraph := conflictgraph.NewUndirectedGraph()
	for i, tx := range txs {
		undiConfGraph.AddVertex(tx.Hash(), uint(i))
	}

	for i := 0; i < txs.Len(); i++ {
		for j := i + 1; j < txs.Len(); j++ {
			if predictLists[i].HasConflict(*predictLists[j]) {
				undiConfGraph.AddEdge(uint(i), uint(j))
			}
		}
	}

	// TODO: 依据 OldAccessList 建图

	OldundiConfGraph := conflictgraph.NewUndirectedGraph()
	for i, tx := range txs {
		undiConfGraph.AddVertex(tx.Hash(), uint(i))
	}

	for i := 0; i < txs.Len(); i++ {
		for j := i + 1; j < txs.Len(); j++ {
			if predictOldLists[i].HasOldConflict(*predictOldLists[j]) {
				OldundiConfGraph.AddEdge(uint(i), uint(j))
			}
		}
	}

	// 对两类图分别输出结果
	groups := undiConfGraph.GetConnectedComponents()
	fmt.Println("Number of Groups:", len(groups))
	for i := 0; i < len(groups); i++ {
		fmt.Printf("Number of Group[%d]:%d\n", i, len(groups[i]))
	}

	Oldgroups := OldundiConfGraph.GetConnectedComponents()
	fmt.Println("Old Number of Groups:", len(Oldgroups))
	for i := 0; i < len(Oldgroups); i++ {
		fmt.Printf("Old Number of Group[%d]:%d\n", i, len(Oldgroups[i]))
	}

	// ! 最大独立集消组模拟
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
