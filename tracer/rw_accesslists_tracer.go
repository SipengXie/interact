package tracer

import (
	"crypto/sha256"
	"math/big"

	"interact/accesslist"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	CODE     = common.Hash(sha256.Sum256([]byte("code")))
	CODEHASH = common.Hash(sha256.Sum256([]byte("codeHash")))
	BALANCE  = common.Hash(sha256.Sum256([]byte("balance")))
	NONCE    = common.Hash(sha256.Sum256([]byte("nonce")))
	ALIVE    = common.Hash(sha256.Sum256([]byte("alive")))
)

// Tracer mainly records the accesslist of each transaction during vm execution (interpreter.run)
type RW_AccessListsTracer struct {
	excl map[common.Address]struct{} // only excludes those stateless precompile contracts
	list *accesslist.RWSet
}

func NewRWAccessListTracer(RWSets *accesslist.RWSet, precompiles []common.Address) *RW_AccessListsTracer {
	excl := make(map[common.Address]struct{})
	for _, addr := range precompiles {
		excl[addr] = struct{}{}
	}
	rwList := accesslist.NewRWSet()
	if RWSets != nil {
		for key := range RWSets.ReadSet {
			addr := common.BytesToAddress(key[:20])
			if _, ok := excl[addr]; !ok {
				rwList.ReadSet.Add(addr, common.BytesToHash(key[20:]))
			}
		}
		for key := range RWSets.WriteSet {
			addr := common.BytesToAddress(key[:20])
			if _, ok := excl[addr]; !ok {
				rwList.WriteSet.Add(addr, common.BytesToHash(key[20:]))
			}
		}
	}
	return &RW_AccessListsTracer{
		excl: excl,
		list: rwList,
	}
}

func (a *RW_AccessListsTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
}

// CaptureState captures all opcodes that touch storage or addresses and adds them to the accesslist.
func (a *RW_AccessListsTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	stack := scope.Stack
	stackData := stack.Data()
	stackLen := len(stackData)
	switch op {
	case vm.SLOAD:
		{
			if stackLen >= 1 {
				slot := common.Hash(stackData[stackLen-1].Bytes32())
				a.list.AddReadSet(scope.Contract.Address(), slot)
			}
		}
	case vm.SSTORE:
		{
			if stackLen >= 1 {
				slot := common.Hash(stackData[stackLen-1].Bytes32())
				a.list.AddWriteSet(scope.Contract.Address(), slot)
			}
		}
	case vm.EXTCODECOPY: // read code
		{
			if stackLen >= 1 {
				addr := common.Address(stackData[stackLen-1].Bytes20())
				if _, ok := a.excl[addr]; !ok {
					a.list.AddReadSet(addr, CODE)
				}
			}
		}
	case vm.EXTCODEHASH:
		{
			if stackLen >= 1 {
				addr := common.Address(stackData[stackLen-1].Bytes20())
				if _, ok := a.excl[addr]; !ok {
					a.list.AddReadSet(addr, CODEHASH)
				}
			}
		}
	case vm.EXTCODESIZE:
		{
			if stackLen >= 1 {
				addr := common.Address(stackData[stackLen-1].Bytes20())
				if _, ok := a.excl[addr]; !ok {
					a.list.AddReadSet(addr, CODE)
				}
			}
		}
	case vm.BALANCE:
		{
			if stackLen >= 1 {
				addr := common.Address(stackData[stackLen-1].Bytes20())
				if _, ok := a.excl[addr]; !ok {
					a.list.AddReadSet(addr, BALANCE)
				}
			}
		}
	case vm.SELFDESTRUCT:
		{
			if stackLen >= 1 {
				beneficiary := common.Address(stackData[stackLen-1].Bytes20())
				if _, ok := a.excl[beneficiary]; !ok {
					a.list.AddReadSet(beneficiary, BALANCE)
					a.list.AddWriteSet(beneficiary, BALANCE)
				}
				addr := scope.Contract.Address()
				if _, ok := a.excl[addr]; !ok {
					a.list.AddWriteSet(addr, BALANCE)
					a.list.AddWriteSet(addr, ALIVE)
				}
			}
		}
	case vm.CALL:
		{
			if stackLen >= 5 {
				from := scope.Contract.Address()
				to := common.Address(stackData[stackLen-2].Bytes20())
				value := stackData[stackLen-3].ToBig()
				if _, ok := a.excl[from]; !ok {
					a.list.AddReadSet(from, BALANCE)
					a.list.AddWriteSet(from, BALANCE)
					a.list.AddReadSet(from, NONCE)
					a.list.AddWriteSet(from, NONCE)
				}
				if _, ok := a.excl[to]; !ok {
					a.list.AddReadSet(to, CODE)
					a.list.AddReadSet(to, CODEHASH)
					// if value == 0, we could determine thta to-balance won't be touched
					if value.Cmp(common.Big0) != 0 {
						a.list.AddReadSet(to, BALANCE)
						a.list.AddWriteSet(to, BALANCE)
					}
				}
			}
		}
	case vm.STATICCALL, vm.DELEGATECALL, vm.CALLCODE:
		{
			if stackLen >= 5 {
				to := common.Address(stackData[stackLen-2].Bytes20())
				if _, ok := a.excl[to]; !ok {
					a.list.AddReadSet(to, CODE)
					a.list.AddReadSet(to, CODEHASH)
				}
			}
		}
	case vm.CREATE2: // cannot apply to CREATE, because the addr is dependent on the nonce
		{
			if stackLen >= 4 {
				from := scope.Contract.Address()
				if _, ok := a.excl[from]; !ok {
					a.list.AddReadSet(from, BALANCE)
					a.list.AddWriteSet(from, BALANCE)
					a.list.AddReadSet(from, NONCE)
					a.list.AddWriteSet(from, NONCE)
				}

				offset, size := stackData[stackLen-2].Uint64(), stackData[stackLen-3].Uint64()
				salt := stackData[stackLen-4].Bytes32()
				input := scope.Memory.GetCopy(int64(offset), int64(size))
				codeHash := crypto.Keccak256Hash(input)
				addr := crypto.CreateAddress2(scope.Contract.Address(), salt, codeHash.Bytes())
				if _, ok := a.excl[addr]; !ok {
					a.list.AddWriteSet(addr, BALANCE)
					a.list.AddWriteSet(addr, CODEHASH)
					a.list.AddWriteSet(addr, CODE)
					a.list.AddWriteSet(addr, NONCE)
					a.list.AddWriteSet(addr, ALIVE)
					// Read to check if the contract addr is already occupied
					a.list.AddReadSet(addr, NONCE)
					a.list.AddReadSet(addr, CODEHASH)
				}
			}
		}
	}
}

func (*RW_AccessListsTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
}

func (*RW_AccessListsTracer) CaptureEnd(output []byte, gasUsed uint64, err error) {}

func (*RW_AccessListsTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
}

func (*RW_AccessListsTracer) CaptureExit(output []byte, gasUsed uint64, err error) {}

func (*RW_AccessListsTracer) CaptureTxStart(gasLimit uint64) {}

func (*RW_AccessListsTracer) CaptureTxEnd(restGas uint64) {}

// AccessList returns the current accesslist maintained by the tracer.
func (a *RW_AccessListsTracer) RWAccessList() *accesslist.RWSet {
	return a.list
}
