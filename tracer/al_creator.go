package tracer

import (
	"fmt"

	"interact/accesslist"
	"interact/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func CreateRWAL(statedb *state.StateDB, tx *types.Transaction, header *types.Header) (*accesslist.RW_AccessLists, error) {
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
		tracer.list.AddReadAL(from, BALANCE)
		tracer.list.AddWriteAL(from, BALANCE)
		tracer.list.AddReadAL(from, NONCE)
		tracer.list.AddWriteAL(from, NONCE)

		tracer.list.AddWriteAL(to, BALANCE)
		tracer.list.AddWriteAL(to, CODEHASH)
		tracer.list.AddWriteAL(to, CODE)
		tracer.list.AddWriteAL(to, NONCE)
		tracer.list.AddWriteAL(to, ALIVE)
		// Read to check if the contract to is already occupied
		tracer.list.AddReadAL(to, NONCE)
		tracer.list.AddReadAL(to, CODEHASH)
	} else {
		tracer.list.AddReadAL(from, BALANCE)
		tracer.list.AddWriteAL(from, BALANCE)
		tracer.list.AddReadAL(from, NONCE)
		tracer.list.AddWriteAL(from, NONCE)

		tracer.list.AddReadAL(to, CODE)
		tracer.list.AddReadAL(to, CODEHASH)

		// if value == 0, we could determine thta to-balance won't be touched
		value := tx.Value()
		if value.Cmp(common.Big0) != 0 {
			tracer.list.AddReadAL(to, BALANCE)
			tracer.list.AddWriteAL(to, BALANCE)
		}

	}
	// if args.RWAccessList() != nil {
	// 	tracer.list.Merge(*args.RWAccessList())
	// }

	// for {
	// RWAL := tracer.RWAccessList()
	// args.AccessList = RWAL.ToJSON()
	// msg, err := args.ToMessage(1000000000000000, header.BaseFee) // 没有设置globalGasCap
	msg, err := core.TransactionToMessage(tx, types.LatestSigner(params.MainnetChainConfig), header.BaseFee)
	msg.SkipAccountChecks = true
	if err != nil {
		return nil, err // TODO: handle error
	}

	coinbase := common.BytesToAddress([]byte("coinbase"))
	// tracer := NewRWAccessListTracer(RWAL, precompiles)
	config := vm.Config{Tracer: tracer}
	txCtx := core.NewEVMTxContext(msg)
	blkCtx := core.NewEVMBlockContext(header, nil, &coinbase)
	vm := vm.NewEVM(blkCtx, txCtx, statedb, params.MainnetChainConfig, config)
	_, err = core.ApplyMessage(vm, msg, new(core.GasPool).AddGas(msg.GasLimit))
	if err != nil {
		return nil, err // TODO: handle error
	}
	return tracer.list, nil
	// if tracer.list.Equal(*tracer.list) {
	// 	return tracer.list
	// }
	// tracer = tracer
	// }
}

func CreateRWALWithTransactions(db *state.StateDB, txs []*types.Transaction, header *types.Header) []*accesslist.RW_AccessLists {
	ret := make([]*accesslist.RW_AccessLists, len(txs))
	for i, tx := range txs {
		fmt.Printf("tx[%d] -%s- starting\n", i, tx.Hash())
		ret[i], _ = CreateRWAL(db, tx, header)
	}
	return ret
}
