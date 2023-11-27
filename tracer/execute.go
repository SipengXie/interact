package tracer

import (
	"fmt"
	"interact/accesslist"
	cachestate "interact/cacheState"
	"interact/core"
	"interact/fullstate"
	"sync"
	"time"

	"github.com/alitto/pond"
	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/panjf2000/ants/v2"
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
		if !statedb.(*cachestate.CacheState).StateJudge {
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
			err := ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			fmt.Println(err)
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

type ParameterForTxGroup struct {
	TxsGroup   types.Transactions
	CacheState *cachestate.CacheState
	Header     *types.Header
	ChainCtx   core.ChainContext
}

// Execute with ants FuncPool with cacheState
func ExecuteWithAntsCacheState(pool *ants.PoolWithFunc, txsGroups []types.Transactions, CacheStates cachestate.CacheStateList, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) {

	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		// Submit tasks to the ants pool
		args := &ParameterForTxGroup{
			TxsGroup:   txsGroups[taskNum],
			CacheState: CacheStates[taskNum],
			Header:     header,
			ChainCtx:   chainCtx,
		}
		err := pool.Invoke(args)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Wait for all tasks to complete
	wg.Wait()
}

// Execute with ants Pool with cacheState
func ExecuteWithAntsPool(pool *ants.Pool, txsGroups []types.Transactions, CacheStates cachestate.CacheStateList, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) {

	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		err := pool.Submit(func() {
			ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println(err)
			wg.Done() // Mark the task as completed
		}
	}
	// Wait for all tasks to complete
	wg.Wait()
}

// Concurrently execute single transaction, rather than transaction groups
func ExecuteWithAntsCacheStateRoundByRound(pool *ants.Pool, txs types.Transactions, CacheStates []*cachestate.CacheState, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) {

	for i := 0; i < len(txs); i++ {
		taskNum := i
		evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, CacheStates[taskNum], params.MainnetChainConfig, vm.Config{})
		// Submit tasks to the ants pool
		err := pool.Submit(func() {
			executeTx(CacheStates[taskNum], txs[taskNum], header, chainCtx, evm)
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}
	}
	wg.Wait()
}

// marking which tx may be aborted
func ExecWithSnapshotState(pool *ants.Pool, txs types.Transactions, snapshots []*fullstate.FullState, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup, readReserve, writeReserve *accesslist.ReserveSet) []error {
	errs := make([]error, len(txs))
	wg.Add(len(txs))
	for i := 0; i < len(txs); i++ {
		taskNum := i
		evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, snapshots[taskNum], params.MainnetChainConfig, vm.Config{})
		// Submit tasks to the ants pool
		err := pool.Submit(func() {
			rwSet := accesslist.NewRWSet()
			snapshots[taskNum].SetRWSet(rwSet)
			errs[taskNum] = executeTx(snapshots[taskNum], txs[taskNum], header, chainCtx, evm)
			readReserve.Reserve(rwSet.ReadSet, uint(taskNum))
			writeReserve.Reserve(rwSet.WriteSet, uint(taskNum))
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}
	}
	wg.Wait()
	return errs
}

// Execute with pond Pool with cacheState
func ExecuteWithPondCacheState(pool *pond.WorkerPool, txsGroups []types.Transactions, CacheStates []*cachestate.CacheState, header *types.Header, chainCtx core.ChainContext) {
	// Iterate over the txsGroups
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j

		// Submit tasks to the pond pool
		pool.Submit(func() {
			st := time.Now()
			ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			// fmt.Println(err)
			executionTime := time.Since(st)
			fmt.Println("Execution time:", executionTime)
		})
	}

	pool.StopAndWait()
}
