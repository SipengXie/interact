package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"
)

func main() {
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

	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)

	fmt.Println("Highest block: ", num)

	for {
		bh := rawdb.ReadCanonicalHash(chainDB, num)
		b := rawdb.ReadBlock(chainDB, bh, num)
		_, err = sdbBackend.OpenTrie(b.Root())
		if err != nil {
			break
		}
		num--
	}
	fmt.Println("First Block Without State: ", num)

	// headBlock := rawdb.ReadHeadBlock(chainDB)
	// stateDB, err := state.New(headBlock.Root(), sdbBackend, nil)
	// if err != nil {
	// 	panic(err)
	// }

	// address := common.HexToAddress("a85C921965A38d98D35ED63BA55eef4F57428E67")
	// balance := stateDB.GetBalance(address)
	// fmt.Println(balance)
}
