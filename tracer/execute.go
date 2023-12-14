package tracer

import (
	"fmt"
	"interact/accesslist"
	"interact/core"
	"interact/state"
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/panjf2000/ants/v2"
)

// This function execute without generating tracer.list
func executeTx(statedb state.StateInterface, tx *types.Transaction, header *types.Header, chainCtx core.ChainContext, evm *vm.EVM) error {
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

	case *state.CacheState:
		if !statedb.(*state.CacheState).StateJudge {
			statedb.(*state.CacheState).StateJudge = true
			// This error means the prediction is false, and the transaction should be reverted
			statedb.RevertToSnapshot(snapshot)
			return ErrFalsePredict
		}

	case *state.StateWithRwSets:
		innerState := statedb.(*state.StateWithRwSets).GetStateDB()
		switch innerState.(type) {
		case *state.CacheState:
			if !innerState.(*state.CacheState).StateJudge {
				innerState.(*state.CacheState).StateJudge = true
				// This error means the prediction is false, and the transaction should be reverted
				statedb.RevertToSnapshot(snapshot)
				return ErrFalsePredict
			}
		default:
			break
		}

	default:
		break
	}

	return nil
}

// ExecuteTxs a batch of transactions in a single atomic state transition.
func ExecuteTxs(sdb state.StateInterface, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) []error {
	evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, sdb, params.MainnetChainConfig, vm.Config{})
	errs := make([]error, len(txs))
	for i, tx := range txs {
		// ExecBasedOnRWSets includes the snapshot logic
		errs[i] = executeTx(sdb, tx, header, chainCtx, evm)
	}
	return errs
}

type ParameterForTxGroup struct {
	TxsGroup   types.Transactions
	CacheState *state.CacheState
	Header     *types.Header
	ChainCtx   core.ChainContext
}

// Execute with ants FuncPool with cacheState
func ExecuteWithAntsCacheState(pool *ants.PoolWithFunc, txsGroups []types.Transactions, CacheStates state.CacheStateList, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) {

	wg.Add(len(txsGroups))
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
func ExecuteWithAntsPool(pool *ants.Pool, txsGroups []types.Transactions, CacheStates state.CacheStateList, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) [][]error {
	wg.Add(len(txsGroups))
	errss := make([][]error, len(txsGroups))
	for j := 0; j < len(txsGroups); j++ {
		taskNum := j
		err := pool.Submit(func() {
			errss[taskNum] = ExecuteTxs(CacheStates[taskNum], txsGroups[taskNum], header, chainCtx)
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println(err)
			wg.Done() // Mark the task as completed
		}
	}
	// Wait for all tasks to complete
	wg.Wait()
	return errss
}

// Concurrently execute single transaction, rather than transaction groups
func ExecuteWithAntsCacheStateRoundByRound(pool *ants.Pool, txs types.Transactions, CacheStates []*state.CacheState, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup) ([]error, accesslist.RWSetList) {
	wg.Add(len(txs))
	errs := make([]error, txs.Len())
	rwsets := make([]*accesslist.RWSet, txs.Len())
	for i := 0; i < len(txs); i++ {
		taskNum := i
		stateWithRwsets := state.NewStateWithRwSets(CacheStates[taskNum])
		rwSet := accesslist.NewRWSet()
		stateWithRwsets.SetRWSet(rwSet)
		evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, stateWithRwsets, params.MainnetChainConfig, vm.Config{})

		// Submit tasks to the ants pool
		err := pool.Submit(func() {
			errs[taskNum] = executeTx(stateWithRwsets, txs[taskNum], header, chainCtx, evm)
			rwsets[taskNum] = rwSet
			wg.Done() // Mark the task as completed
		})
		if err != nil {
			fmt.Println("Error submitting task to ants pool:", err)
			wg.Done() // Mark the task as completed
		}
	}
	wg.Wait()
	return errs, rwsets
}

// txs is the whole transactions of a block
// txsIndex speicifies the index of transactions to be executed
func ExecWithSnapshotState(pool *ants.Pool, txs types.Transactions, txsIndex []int, snapshots []*state.StateWithRwSets, header *types.Header, chainCtx core.ChainContext, wg *sync.WaitGroup, readReserve, writeReserve *accesslist.ReserveSet) []error {
	errs := make([]error, len(txsIndex))
	wg.Add(len(txsIndex))
	for i := 0; i < len(txsIndex); i++ {
		taskNum := i
		evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, snapshots[taskNum], params.MainnetChainConfig, vm.Config{})
		// Submit tasks to the ants pool
		err := pool.Submit(func() {
			rwSet := accesslist.NewRWSet()
			snapshots[taskNum].SetRWSet(rwSet)
			index := txsIndex[taskNum]
			errs[taskNum] = executeTx(snapshots[taskNum], txs[index], header, chainCtx, evm)
			readReserve.Reserve(rwSet.ReadSet, uint(txsIndex[taskNum]))
			writeReserve.Reserve(rwSet.WriteSet, uint(txsIndex[taskNum]))
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
