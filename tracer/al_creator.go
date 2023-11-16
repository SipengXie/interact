package tracer

import (
	"errors"
	"interact/accesslist"
	"interact/core"
	"interact/fullstate"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
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

// ------------------------------------------------------
// CreateOldAL 获取交易实际运行时的OldAccessList
func ExecBasedOnOldAL(statedb vm.StateDB, tx *types.Transaction, header *types.Header, chainCtx core.ChainContext) (*accesslist.AccessList, error) {
	from, _ := types.Sender(types.LatestSigner(params.MainnetChainConfig), tx)
	var to common.Address = common.Address{}
	if tx.To() != nil {
		to = *tx.To()
	}
	if to == (common.Address{}) {
		to = crypto.CreateAddress(from, tx.Nonce())
	}
	isPostMerge := header.Difficulty.Cmp(common.Big0) == 0
	precompiles := vm.ActivePrecompiles(params.MainnetChainConfig.Rules(header.Number, isPostMerge, header.Time)) // 非常不严谨的chainconfig

	// 新建OldAccessList追踪器
	tracer := NewAccessListTracer(nil, from, to, precompiles)

	// 开始执行交易步骤
	msg, err := core.TransactionToMessage(tx, types.LatestSigner(params.MainnetChainConfig), header.BaseFee)

	if err != nil {
		return nil, err // TODO: handle error
	}
	msg.SkipAccountChecks = true
	coinbase := common.BytesToAddress([]byte("coinbase"))

	config := vm.Config{Tracer: tracer}
	txCtx := core.NewEVMTxContext(msg)
	blkCtx := core.NewEVMBlockContext(header, chainCtx, &coinbase)
	vm := vm.NewEVM(blkCtx, txCtx, statedb, params.MainnetChainConfig, config)
	_, err = core.ApplyMessage(vm, msg, new(core.GasPool).AddGas(msg.GasLimit))
	if err != nil {
		return nil, err // TODO: handle error
	}

	// tracer.list就是交易实际执行时获取到的OldAccessList(tracer格式)，进行格式转换，转为OldAccessList形式
	return ChangeAccessList(tracer.list), nil
}

// ChangeAccessList tracer.AccessList -> AccessList 类型转换函数
// type accessList map[common.Address]accessListSlots   type accessListSlots map[common.Hash]struct{}
// ->
//
//	type AccessList struct {
//	    Addresses map[common.Address]int
//	    Slots     []map[common.Hash]struct{}
//	}
func ChangeAccessList(tracer accessList) *accesslist.AccessList {
	// 后来实现
	index := 0
	al := accesslist.NewAccessList() // 新建OldAccessList
	for key, value := range tracer {
		// key是address，value是accessListSlots
		// 第一步：判断有没有accessListSlots
		if value == nil || len(value) == 0 {
			// 只有address没有slots
			al.AddAddress(key) // 添加地址，默认int = -1
		} else {
			// 有address和对应的slots
			al.AddAddressInt(key, index)
			index++ // 自增
			// 处理Slots
			al.Slots[index] = make(map[common.Hash]struct{}, 0)
			for s, st := range value {
				al.Slots[index][s] = st
			}
		}
	}
	return al
}

func CreateOldALWithTransactions(db *state.StateDB, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) ([]*accesslist.AccessList, []error) {
	ret := make([]*accesslist.AccessList, len(txs))
	err := make([]error, len(txs))
	for i, tx := range txs {
		ret[i], err[i] = ExecBasedOnOldAL(db, tx, header, chainCtx)
	}
	return ret, err
}
