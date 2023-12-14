package state

import (
	"interact/accesslist"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type StateWithRwSets struct {
	stateDB StateInterface
	rwSets  *accesslist.RWSet
}

func NewStateWithRwSets(stateDB StateInterface) *StateWithRwSets {
	return &StateWithRwSets{
		stateDB: stateDB,
		rwSets:  nil,
	}
}

// ----------------------- Getters ----------------------------
func (fs *StateWithRwSets) GetStateDB() StateInterface {
	return fs.stateDB
}

func (fs *StateWithRwSets) GetRWSet() *accesslist.RWSet {
	return fs.rwSets
}

func (fs *StateWithRwSets) GetBalance(addr common.Address) *big.Int {
	if fs.rwSets != nil {
		// fs.rwSets.AddReadSet(addr, accesslist.BALANCE)
	}
	return fs.stateDB.GetBalance(addr)
}

func (fs *StateWithRwSets) GetNonce(addr common.Address) uint64 {
	if fs.rwSets != nil {
		// fs.rwSets.AddReadSet(addr, accesslist.NONCE)
	}
	return fs.stateDB.GetNonce(addr)
}

func (fs *StateWithRwSets) GetCodeHash(addr common.Address) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODEHASH)
	}
	return fs.stateDB.GetCodeHash(addr)
}

func (fs *StateWithRwSets) GetCode(addr common.Address) []byte {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODE)
	}
	return fs.stateDB.GetCode(addr)
}

func (fs *StateWithRwSets) GetCodeSize(addr common.Address) int {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODE)
	}
	return fs.stateDB.GetCodeSize(addr)
}

func (fs *StateWithRwSets) GetRefund() uint64 {
	return fs.stateDB.GetRefund()
}

func (fs *StateWithRwSets) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetCommittedState(addr, key)
}

func (fs *StateWithRwSets) GetState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetState(addr, key)
}

func (fs *StateWithRwSets) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetTransientState(addr, key)
}

func (fs *StateWithRwSets) HasSelfDestructed(addr common.Address) bool {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.ALIVE)
	}
	return fs.HasSelfDestructed(addr)
}

func (fs *StateWithRwSets) Exist(addr common.Address) bool {
	return fs.stateDB.Exist(addr)
}

func (fs *StateWithRwSets) Empty(addr common.Address) bool {
	return fs.stateDB.Empty(addr)
}

// ----------------------- Setters ----------------------------
func (fs *StateWithRwSets) SetStateDB(stateDB *state.StateDB) {
	fs.stateDB = stateDB
}

func (fs *StateWithRwSets) SetRWSet(rwSets *accesslist.RWSet) {
	fs.rwSets = rwSets
}

func (fs *StateWithRwSets) CreateAccount(addr common.Address) {
	fs.stateDB.CreateAccount(addr)
}

func (fs *StateWithRwSets) AddBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		// fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.AddBalance(addr, amount)
}

func (fs *StateWithRwSets) SubBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		// fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SubBalance(addr, amount)
}

func (fs *StateWithRwSets) SetBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		// fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SetBalance(addr, amount)
}

func (fs *StateWithRwSets) SetNonce(addr common.Address, nonce uint64) {
	if fs.rwSets != nil {
		// fs.rwSets.AddWriteSet(addr, accesslist.NONCE)
	}
	fs.stateDB.SetNonce(addr, nonce)
}

func (fs *StateWithRwSets) SetCode(addr common.Address, code []byte) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.CODE)
		fs.rwSets.AddWriteSet(addr, accesslist.CODEHASH)
	}
	fs.stateDB.SetCode(addr, code)
}

func (fs *StateWithRwSets) SetState(addr common.Address, key, value common.Hash) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, key)
	}
	fs.stateDB.SetState(addr, key, value)
}

func (fs *StateWithRwSets) SetTransientState(addr common.Address, key, value common.Hash) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, key)
	}
	fs.stateDB.SetTransientState(addr, key, value)
}

func (fs *StateWithRwSets) SelfDestruct(addr common.Address) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.ALIVE)
		fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SelfDestruct(addr)
}

func (fs *StateWithRwSets) Selfdestruct6780(addr common.Address) {
	fs.stateDB.Selfdestruct6780(addr)
}

// ----------------------Functional Methods---------------------
func (fs *StateWithRwSets) AddRefund(gas uint64) {
	fs.stateDB.AddRefund(gas)
}

func (fs *StateWithRwSets) SubRefund(gas uint64) {
	fs.stateDB.SubRefund(gas)
}

// AddAddressToAccessList adds the given address to the access list
func (fs *StateWithRwSets) AddAddressToAccessList(addr common.Address) {
	fs.stateDB.AddAddressToAccessList(addr)
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (fs *StateWithRwSets) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	fs.stateDB.AddSlotToAccessList(addr, slot)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (fs *StateWithRwSets) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return fs.stateDB.SlotInAccessList(addr, slot)
}

func (fs *StateWithRwSets) RevertToSnapshot(revid int) {
	fs.stateDB.RevertToSnapshot(revid)
}

func (fs *StateWithRwSets) Snapshot() int {
	return fs.stateDB.Snapshot()
}

func (fs *StateWithRwSets) AddLog(log *types.Log) {
	fs.stateDB.AddLog(log)
}

func (fs *StateWithRwSets) AddPreimage(hash common.Hash, preimage []byte) {
	fs.stateDB.AddPreimage(hash, preimage)
}

func (fs *StateWithRwSets) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
	fs.stateDB.Prepare(rules, sender, coinbase, dst, precompiles, list)
}

func (fs *StateWithRwSets) AddressInAccessList(addr common.Address) bool {
	return fs.stateDB.AddressInAccessList(addr)
}

func (fs *StateWithRwSets) SetTxContext(thash common.Hash, ti int) {
	fs.stateDB.SetTxContext(thash, ti)
}
