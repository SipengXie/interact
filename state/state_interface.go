package state

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type StateInterface interface {
	vm.StateDB
	SetBalance(common.Address, *big.Int)
	SetTxContext(common.Hash, int)
}

type StateList []StateInterface
