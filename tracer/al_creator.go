package tracer

import (
	"fmt"
	"math/big"

	"interact/accesslist"

	"interact/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func CreateRWAL(db *state.StateDB, args TransactionArgs, header *types.Header, isCopied bool) *accesslist.RW_AccessLists {
	from := args.from()
	to := args.to()
	isCreate := false
	if to == (common.Address{}) {
		hash := crypto.Keccak256Hash(args.data()).Bytes()
		to = crypto.CreateAddress2(from, args.salt().Bytes32(), hash)
		isCreate = true
	}
	isPostMerge := header.Difficulty.Cmp(common.Big0) == 0
	precompiles := vm.ActivePrecompiles(params.MainnetChainConfig.Rules(header.Number, isPostMerge, header.Time)) // 非常不严谨的chainconfig
	prevTracer := NewRWAccessListTracer(nil, precompiles)
	if isCreate {
		prevTracer.list.AddReadAL(from, BALANCE)
		prevTracer.list.AddWriteAL(from, BALANCE)
		prevTracer.list.AddReadAL(from, NONCE)
		prevTracer.list.AddWriteAL(from, NONCE)

		prevTracer.list.AddWriteAL(to, BALANCE)
		prevTracer.list.AddWriteAL(to, CODEHASH)
		prevTracer.list.AddWriteAL(to, CODE)
		prevTracer.list.AddWriteAL(to, NONCE)
		prevTracer.list.AddWriteAL(to, ALIVE)
		// Read to check if the contract to is already occupied
		prevTracer.list.AddReadAL(to, NONCE)
		prevTracer.list.AddReadAL(to, CODEHASH)
	} else {
		prevTracer.list.AddReadAL(from, BALANCE)
		prevTracer.list.AddWriteAL(from, BALANCE)
		prevTracer.list.AddReadAL(from, NONCE)
		prevTracer.list.AddWriteAL(from, NONCE)

		prevTracer.list.AddReadAL(to, CODE)
		prevTracer.list.AddReadAL(to, CODEHASH)

		// if value == 0, we could determine thta to-balance won't be touched
		value, _ := new(big.Int).SetString(args.Value, 10)
		if value.Cmp(common.Big0) != 0 {
			prevTracer.list.AddReadAL(to, BALANCE)
			prevTracer.list.AddWriteAL(to, BALANCE)
		}

	}
	if args.RWAccessList() != nil {
		prevTracer.list.Merge(*args.RWAccessList())
	}

	for {
		RWAL := prevTracer.RWAccessList()
		var statedb *state.StateDB
		if !isCopied {
			statedb = db.Copy()
		} else {
			statedb = db
		}

		args.AccessList = RWAL.ToJSON()
		msg, err := args.ToMessage(1000000000000000, header.BaseFee) // 没有设置globalGasCap
		if err != nil {
			panic(err) // TODO: handle error
		}

		coinbase := common.BytesToAddress([]byte("coinbase"))
		tracer := NewRWAccessListTracer(RWAL, precompiles)
		config := vm.Config{Tracer: tracer, NoBaseFee: true}
		txCtx := core.NewEVMTxContext(msg)
		blkCtx := core.NewEVMBlockContext(header, nil, &coinbase)
		vm := vm.NewEVM(blkCtx, txCtx, statedb, params.MainnetChainConfig, config)
		res, err := core.ApplyMessage(vm, msg, new(core.GasPool).AddGas(msg.GasLimit))
		if err != nil {
			panic(err) // TODO: handle error
		}
		fmt.Println("used gas:", res.UsedGas)
		if tracer.list.Equal(*prevTracer.list) {
			return tracer.list
		}
		prevTracer = tracer
	}
}

func CreateRWALWithTransactions(db *state.StateDB, args []TransactionArgs, header *types.Header) []*accesslist.RW_AccessLists {
	dbCopy := db.Copy()
	ret := make([]*accesslist.RW_AccessLists, len(args))
	for _, tx := range args {
		ret = append(ret, CreateRWAL(dbCopy, tx, header, true))
	}
	return ret
}
