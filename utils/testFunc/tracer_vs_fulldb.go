package testfunc

import (
	"fmt"
	"interact/core"
	"interact/fullstate"
	"interact/tracer"
	"interact/utils"
	"math/rand"
	"os"
	"time"

	statedb "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
)

// CompareTracerAndFulldb compare two manners to predict rw al sets
func CompareTracerAndFulldb(chainDB ethdb.Database, sdbBackend statedb.Database, num uint64) {
	fakeChainCtx := core.NewFakeChainContext(chainDB)
	baseState, _ := utils.GetState(chainDB, sdbBackend, num-1)
	block, header := utils.GetBlockAndHeader(chainDB, num)

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
