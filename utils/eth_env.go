package utils

import (
	"github.com/ethereum/go-ethereum/core/rawdb"
	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/pathdb"
)

// GetEthDatabaseAndStateDatabase get node, ethdb and state database(eth env)
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

// GetState get StateDB from block[num].Root
func GetState(chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) (*statedb.StateDB, error) {
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num)
	return statedb.New(baseHeader.Root, sdbBackend, nil)
}

// GetBlockAndHeader get a block and its header with blockNum
func GetBlockAndHeader(chainDB ethdb.Database, num uint64) (*types.Block, *types.Header) {
	headHash := rawdb.ReadCanonicalHash(chainDB, num)
	header := rawdb.ReadHeader(chainDB, headHash, num)
	block := rawdb.ReadBlock(chainDB, headHash, num)
	return block, header
}
