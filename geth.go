package main

import (
	"fmt"
	"interact/tracer"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
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

func PredictRWAL(tx *types.Transaction, chainDB ethdb.Database, sdbBackend state.Database, num uint64) {
	txArgs := tracer.NewTransactionArgs(tx)

	baseHead := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHead, num-1)

	state, err := state.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	list := tracer.CreateRWAL(state, txArgs, baseHeader, false)
	listJSON := list.ToJSON()
	b := common.Hex2Bytes(listJSON)
	fmt.Println("Tx Hash is:", tx.Hash())
	fmt.Println(string(b))
}

func PredictRWALs(txs []*types.Transaction, chainDB ethdb.Database, sdbBackend state.Database, num uint64) {
	args := make([]tracer.TransactionArgs, len(txs))
	for i, tx := range txs {
		args[i] = tracer.NewTransactionArgs(tx)
	}
	baseHead := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHead, num-1)

	state, err := state.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		panic(err)
	}

	lists := tracer.CreateRWALWithTransactions(state, args, baseHeader)
	for id, list := range lists {
		listJSON := list.ToJSON()
		b := common.Hex2Bytes(listJSON)
		fmt.Println("Tx Hash is:", txs[id].Hash())
		fmt.Println("RWAL is:", string(b))
	}
}

func main() {
	chainDB, sdbBackend := GetEthDatabaseAndStateDatabase()
	head := rawdb.ReadHeadBlockHash(chainDB)
	num := *rawdb.ReadHeaderNumber(chainDB, head)
	headBlock := rawdb.ReadBlock(chainDB, head, num)
	txs := headBlock.Transactions()
	// PredictRWALs(txs, chainDB, sdbBackend, num)

	tx := txs[1]
	PredictRWAL(tx, chainDB, sdbBackend, num)
}
