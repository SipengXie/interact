package main

import (
	"fmt"
	"interact/accesslist"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	"interact/mis"
	"interact/tracer"
	"os"

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
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	list, err := tracer.CreateRWAL(state, tx, header, fakeChainCtx)
	if err != nil {
		fmt.Println("NIL tx hash:", tx.Hash())
	}
	// listJSON := list.ToJSON()
	// b := common.Hex2Bytes(listJSON)
	// fmt.Println("Tx Hash is:", tx.Hash())
	// fmt.Println(string(b))

	return list
}

func TrueRWALs(txs []*types.Transaction, chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) ([]*accesslist.RW_AccessLists, error) {
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := statedb.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		return nil, err
	}

	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	fakeChainCtx := core.NewFakeChainContext(chainDB)

	lists, errs := tracer.CreateRWALWithTransactions(state, txs, header, fakeChainCtx)
	for i, err := range errs {
		if err != nil {
			fmt.Println("In TRUERWALS, tx hash:", txs[i].Hash())
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

		trueLists, err := TrueRWALs(txs, chainDB, sdbBackend, num)
		if err != nil {
			break
		}

		predictLists := make([]*accesslist.RW_AccessLists, txs.Len())
		for i, tx := range txs {
			predictLists[i] = PredictRWAL(tx, chainDB, sdbBackend, num)
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

func main() {
	Node, chainDB, sdbBackend := GetEthDatabaseAndStateDatabase()
	defer Node.Close()

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)

	IterateBlock(chainDB, sdbBackend, num)
}
