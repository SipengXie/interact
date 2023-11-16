package tracer

import (
	"errors"
	"interact/accesslist"
	"interact/core"
	"interact/fullstate"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var ErrFalsePredict error = errors.New("False Predict List")

func addCreate(tracer *RW_AccessListsTracer, from, to common.Address) {
	tracer.list.AddReadSet(from, BALANCE)
	tracer.list.AddWriteSet(from, BALANCE)
	tracer.list.AddReadSet(from, NONCE)
	tracer.list.AddWriteSet(from, NONCE)

	tracer.list.AddWriteSet(to, BALANCE)
	tracer.list.AddWriteSet(to, CODEHASH)
	tracer.list.AddWriteSet(to, CODE)
	tracer.list.AddWriteSet(to, NONCE)
	tracer.list.AddWriteSet(to, ALIVE)
	// Read to check if the contract to is already occupied
	tracer.list.AddReadSet(to, NONCE)
	tracer.list.AddReadSet(to, CODEHASH)
}

func addCall(tracer *RW_AccessListsTracer, from, to common.Address, value *big.Int) {
	tracer.list.AddReadSet(from, BALANCE)
	tracer.list.AddWriteSet(from, BALANCE)
	tracer.list.AddReadSet(from, NONCE)
	tracer.list.AddWriteSet(from, NONCE)

	tracer.list.AddReadSet(to, CODE)
	tracer.list.AddReadSet(to, CODEHASH)

	// if value == 0, we could determine thta to-balance won't be touched
	if value.Cmp(common.Big0) != 0 {
		tracer.list.AddReadSet(to, BALANCE)
		tracer.list.AddWriteSet(to, BALANCE)
	}
}

func PredictWithTracer(statedb vm.StateDB, tx *types.Transaction, header *types.Header, chainCtx core.ChainContext) (*accesslist.RWSet, error) {
	from, _ := types.Sender(types.LatestSigner(params.MainnetChainConfig), tx)
	var to common.Address = common.Address{}
	if tx.To() != nil {
		to = *tx.To()
	}
	isCreate := false
	if to == (common.Address{}) {
		// hash := crypto.Keccak256Hash(tx.Data()).Bytes()
		// to = crypto.CreateAddress2(from, args.salt().Bytes32(), hash)
		to = crypto.CreateAddress(from, tx.Nonce())
		isCreate = true
	}
	isPostMerge := header.Difficulty.Cmp(common.Big0) == 0
	precompiles := vm.ActivePrecompiles(params.MainnetChainConfig.Rules(header.Number, isPostMerge, header.Time)) // 非常不严谨的chainconfig
	tracer := NewRWAccessListTracer(nil, precompiles)

	if isCreate {
		addCreate(tracer, from, to)
	} else {
		addCall(tracer, from, to, tx.Value())
	}

	evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, statedb, params.MainnetChainConfig, vm.Config{Tracer: tracer})
	err := executeTx(statedb, tx, header, chainCtx, evm)
	if err != nil {
		return nil, err
	}
	return tracer.list, nil
}

// -------------------------------------------------------
func ExecToGenerateRWSet(fulldb *fullstate.FullState, tx *types.Transaction, header *types.Header, chainCtx core.ChainContext) (*accesslist.RWSet, error) {
	rwSet := accesslist.NewRWSet()
	fulldb.SetRWSet(rwSet)
	evm := vm.NewEVM(core.NewEVMBlockContext(header, chainCtx, &header.Coinbase), vm.TxContext{}, fulldb, params.MainnetChainConfig, vm.Config{})
	err := executeTx(fulldb, tx, header, chainCtx, evm)
	if err != nil {
		return nil, err
	}
	return rwSet, nil
}

func CreateRWSetsWithTransactions(db *fullstate.FullState, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) ([]*accesslist.RWSet, []error) {
	ret := make([]*accesslist.RWSet, len(txs))
	err := make([]error, len(txs))
	for i, tx := range txs {
		ret[i], err[i] = ExecToGenerateRWSet(db, tx, header, chainCtx)
	}
	return ret, err
}
