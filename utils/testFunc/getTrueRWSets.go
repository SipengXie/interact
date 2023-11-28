package testfunc

import (
	"fmt"
	"interact/accesslist"
	"interact/core"
	interactState "interact/state"
	"interact/tracer"

	"github.com/ethereum/go-ethereum/core/rawdb"
	ethState "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

func TrueRWSets(txs types.Transactions, chainDB ethdb.Database, sdbBackend ethState.Database, num uint64) (accesslist.RWSetList, error) {
	baseHeadHash := rawdb.ReadCanonicalHash(chainDB, num-1)
	baseHeader := rawdb.ReadHeader(chainDB, baseHeadHash, num-1)

	state, err := ethState.New(baseHeader.Root, sdbBackend, nil)
	if err != nil {
		return nil, err
	}
	fulldb := interactState.NewFullState(state)

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
