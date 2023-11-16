package tracer

import (
	cachestate "interact/cacheState"
	"interact/core"

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
func ExecuteWithGopoolCacheState(txsGroups []types.Transactions, CacheStates []*cachestate.CacheState, header *types.Header, chainCtx core.ChainContext) {
	// Initialize a GoPool
	pool := gopool.NewGoPool(16, gopool.WithTaskQueueSize(len(txsGroups)), gopool.WithMinWorkers(8), gopool.WithResultCallback(func(result interface{}) {
	}))
	defer pool.Release()
	// Add tasks to the pool
	// !!! Gopool will costs 50ms to do the scheduling !!!
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		pool.AddTask(func() (interface{}, error) {
			errs := ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			return errs, nil
		})
	}
	pool.Wait()
}

// Execute with GoPool with StatetDB
func ExecuteWithGopoolStateDB(txsGroups []types.Transactions, statedb []*state.StateDB, header *types.Header, chainCtx core.ChainContext) {
	// Initialize a GoPool
	pool := gopool.NewGoPool(16, gopool.WithTaskQueueSize(len(txsGroups)), gopool.WithMinWorkers(8), gopool.WithResultCallback(func(result interface{}) {
		// fmt.Println("Task result:", result)
	}))
	defer pool.Release()
	// Add tasks to the pool
	// !!! Gopool will costs 50ms to do the scheduling !!!
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		pool.AddTask(func() (interface{}, error) {
			errs := ExecuteTxs(statedb[taskNum], txsGroups[taskNum], header, chainCtx)
			return errs, nil
		})
	}
	pool.Wait()
}
