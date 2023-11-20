package tracer

import (
	"fmt"
	cachestate "interact/cacheState"
	"interact/core"
	"time"

	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// This function execute without generating tracer.list
func executeTx(statedb vm.StateDB, tx *types.Transaction, header *types.Header, chainCtx core.ChainContext, evm *vm.EVM) error {
	msg, err := core.TransactionToMessage(tx, types.LatestSigner(params.MainnetChainConfig), header.BaseFee)

	if err != nil {
		// This error means the transaction is invalid and should be discarded
		return err
	}
	// Skip the nonce check!
	msg.SkipAccountChecks = true
	txCtx := core.NewEVMTxContext(msg)
	evm.TxContext = txCtx

	snapshot := statedb.Snapshot()
	_, err = core.ApplyMessage(evm, msg, new(core.GasPool).AddGas(msg.GasLimit))
	if err != nil {
		// This error means the Execution phase failed and the transaction has been reverted
		return err
	}

	switch statedb.(type) {
	case *cachestate.CacheState:
		if statedb.(*cachestate.CacheState).StateJudge == false {
			statedb.(*cachestate.CacheState).StateJudge = true
			// This error means the prediction is false, and the transaction should be reverted
			statedb.RevertToSnapshot(snapshot)
			return ErrFalsePredict
		}
	default:
		break
	}

	return nil
}

// ExecuteTxs a batch of transactions in a single atomic state transition.
func ExecuteTxs(sdb vm.StateDB, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) []error {
	evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, sdb, params.MainnetChainConfig, vm.Config{})
	errs := make([]error, len(txs))
	for i, tx := range txs {
		// ExecBasedOnRWSets includes the snapshot logic
		errs[i] = executeTx(sdb, tx, header, chainCtx, evm)
	}
	return errs
}

// Execute with GoPool with cacheState
func ExecuteWithGopoolCacheState(pool gopool.GoPool, txsGroups []types.Transactions, CacheStates []*cachestate.CacheState, header *types.Header, chainCtx core.ChainContext) {
	// Add tasks to the pool
	// !!! Gopool will costs 50ms to do the scheduling !!!
	// st := time.Now()
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		pool.AddTask(func() (interface{}, error) {
			st := time.Now()
			errs := ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			fmt.Println(errs)
			return time.Since(st), nil
		})
	}
	pool.Wait()
	// fmt.Println("Execute Costs:", time.Since(st))
}

// Execute with GoPool with StatetDB
func ExecuteWithGopoolStateDB(pool gopool.GoPool, txsGroups []types.Transactions, statedb []*state.StateDB, header *types.Header, chainCtx core.ChainContext) {
	// Initialize a GoPool
	// Add tasks to the pool
	// !!! Gopool will costs 50ms to do the scheduling !!!
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		pool.AddTask(func() (interface{}, error) {
			st := time.Now()
			ExecuteTxs(statedb[taskNum], txsGroups[taskNum], header, chainCtx)
			return time.Since(st), nil
		})
	}
	pool.Wait()
}
