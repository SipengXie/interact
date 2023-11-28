package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type State interface {
	vm.StateDB
	SetBalance(common.Address, *big.Int)
	SetTxContext(common.Hash, int)
}

type StateList []State
