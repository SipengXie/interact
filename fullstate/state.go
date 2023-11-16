package fullstate

import (
	"interact/accesslist"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type FullState struct {
	stateDB *state.StateDB
	rwSets  *accesslist.RWSet
}

func NewFullState(stateDB *state.StateDB) *FullState {
	return &FullState{
		stateDB: stateDB,
		rwSets:  nil,
	}
}

// ----------------------- Getters ----------------------------
func (fs *FullState) GetStateDB() *state.StateDB {
	return fs.stateDB
}

func (fs *FullState) GetRWSet() *accesslist.RWSet {
	return fs.rwSets
}

func (fs *FullState) GetBalance(addr common.Address) *big.Int {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.BALANCE)
	}
	return fs.stateDB.GetBalance(addr)
}

func (fs *FullState) GetNonce(addr common.Address) uint64 {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.NONCE)
	}
	return fs.stateDB.GetNonce(addr)
}

func (fs *FullState) GetCodeHash(addr common.Address) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODEHASH)
	}
	return fs.stateDB.GetCodeHash(addr)
}

func (fs *FullState) GetCode(addr common.Address) []byte {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODE)
	}
	return fs.stateDB.GetCode(addr)
}

func (fs *FullState) GetCodeSize(addr common.Address) int {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.CODE)
	}
	return fs.stateDB.GetCodeSize(addr)
}

func (fs *FullState) GetRefund() uint64 {
	return fs.stateDB.GetRefund()
}

func (fs *FullState) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetCommittedState(addr, key)
}

func (fs *FullState) GetState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetState(addr, key)
}

func (fs *FullState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, key)
	}
	return fs.stateDB.GetTransientState(addr, key)
}

func (fs *FullState) HasSelfDestructed(addr common.Address) bool {
	if fs.rwSets != nil {
		fs.rwSets.AddReadSet(addr, accesslist.ALIVE)
	}
	return fs.HasSelfDestructed(addr)
}

func (fs *FullState) Exist(addr common.Address) bool {
	return fs.stateDB.Exist(addr)
}

func (fs *FullState) Empty(addr common.Address) bool {
	return fs.stateDB.Empty(addr)
}

// ----------------------- Setters ----------------------------
func (fs *FullState) SetStateDB(stateDB *state.StateDB) {
	fs.stateDB = stateDB
}

func (fs *FullState) SetRWSet(rwSets *accesslist.RWSet) {
	fs.rwSets = rwSets
}

func (fs *FullState) CreateAccount(addr common.Address) {
	fs.stateDB.CreateAccount(addr)
}

func (fs *FullState) AddBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.AddBalance(addr, amount)
}

func (fs *FullState) SubBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SubBalance(addr, amount)
}

func (fs *FullState) SetBalance(addr common.Address, amount *big.Int) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SetBalance(addr, amount)
}

func (fs *FullState) SetNonce(addr common.Address, nonce uint64) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.NONCE)
	}
	fs.stateDB.SetNonce(addr, nonce)
}

func (fs *FullState) SetCode(addr common.Address, code []byte) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.CODE)
		fs.rwSets.AddWriteSet(addr, accesslist.CODEHASH)
	}
	fs.stateDB.SetCode(addr, code)
}

func (fs *FullState) SetState(addr common.Address, key, value common.Hash) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, key)
	}
	fs.stateDB.SetState(addr, key, value)
}

func (fs *FullState) SetTransientState(addr common.Address, key, value common.Hash) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, key)
	}
	fs.stateDB.SetTransientState(addr, key, value)
}

func (fs *FullState) SelfDestruct(addr common.Address) {
	if fs.rwSets != nil {
		fs.rwSets.AddWriteSet(addr, accesslist.ALIVE)
		fs.rwSets.AddWriteSet(addr, accesslist.BALANCE)
	}
	fs.stateDB.SelfDestruct(addr)
}

func (fs *FullState) Selfdestruct6780(addr common.Address) {
	fs.stateDB.Selfdestruct6780(addr)
}

// ----------------------Functional Methods---------------------
func (fs *FullState) AddRefund(gas uint64) {
	fs.stateDB.AddRefund(gas)
}

func (fs *FullState) SubRefund(gas uint64) {
	fs.stateDB.SubRefund(gas)
}

// AddAddressToAccessList adds the given address to the access list
func (fs *FullState) AddAddressToAccessList(addr common.Address) {
	fs.stateDB.AddAddressToAccessList(addr)
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (fs *FullState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	fs.stateDB.AddSlotToAccessList(addr, slot)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (fs *FullState) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return fs.stateDB.SlotInAccessList(addr, slot)
}

func (fs *FullState) RevertToSnapshot(revid int) {
	fs.stateDB.RevertToSnapshot(revid)
}

func (fs *FullState) Snapshot() int {
	return fs.stateDB.Snapshot()
}

func (fs *FullState) AddLog(log *types.Log) {
	fs.stateDB.AddLog(log)
}

func (fs *FullState) AddPreimage(hash common.Hash, preimage []byte) {
	fs.stateDB.AddPreimage(hash, preimage)
}

func (fs *FullState) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
	fs.stateDB.Prepare(rules, sender, coinbase, dst, precompiles, list)
}

func (fs *FullState) AddressInAccessList(addr common.Address) bool {
	return fs.stateDB.AddressInAccessList(addr)
}

func (fs *FullState) SetTxContext(thash common.Hash, ti int) {
	fs.stateDB.SetTxContext(thash, ti)
}
