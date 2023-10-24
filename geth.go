package main

import (
	"fmt"
	"interact/tracer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"
)

func GetEthDatabaseAndStateDatabase() (ethdb.Database, state.Database) {
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
	sdbBackend := state.NewDatabaseWithNodeDB(chainDB, trieDB)
	return chainDB, sdbBackend
}

func main() {
	chainDB, sdbBackend := GetEthDatabaseAndStateDatabase()
	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)
	headBlock := rawdb.ReadBlock(chainDB, head, num)
	tx := headBlock.Transactions()[1]
	fmt.Println(tx.Hash())

	txArgs := tracer.NewTransactionArgs(headBlock.Transactions()[1])

	baseHead := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHead, num-1)

	state, err := state.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	list := tracer.CreateRWAL(state, txArgs, baseHeader)
	listJSON := list.ToJSON()
	b := common.Hex2Bytes(listJSON)
	fmt.Println(string(b))

	// fmt.Println("Highest block: ", num)

	// for {
	// 	bh := rawdb.ReadCanonicalHash(chainDB, num)
	// 	b := rawdb.ReadBlock(chainDB, bh, num)
	// 	_, err := sdbBackend.OpenTrie(b.Root())
	// 	if err != nil {
	// 		break
	// 	}
	// 	num--
	// }
	// fmt.Println("First Block Without State: ", num)

	// headBlock := rawdb.ReadHeadBlock(chainDB)
	// stateDB, err := state.New(headBlock.Root(), sdbBackend, nil)
	// if err != nil {
	// 	panic(err)
	// }

	// address := common.HexToAddress("a85C921965A38d98D35ED63BA55eef4F57428E67")
	// balance := stateDB.GetBalance(address)
	// fmt.Println(balance)
}
