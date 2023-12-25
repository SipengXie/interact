package state

import (
	"interact/accesslist"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// FullCacheConcurrent for APEX & APEX+
type FullCacheConcurrent struct {
	Accounts    sync.Map // try using sync.Map
	prefectched accesslist.ALTuple

	Logs    map[common.Hash][]*types.Log `json:"logs,omitempty"`
	thash   common.Hash
	txIndex int
	logSize uint
}

func NewFullCacheConcurrent() *FullCacheConcurrent {
	return &FullCacheConcurrent{
		Accounts:    sync.Map{},
		prefectched: make(accesslist.ALTuple),
		Logs:        make(map[common.Hash][]*types.Log),
	}
}

func (s *FullCacheConcurrent) Copy() *FullCacheConcurrent {
	newS := NewFullCacheConcurrent()
	s.Accounts.Range(func(key any, value any) bool {
		newS.Accounts.Store(key, value)
		return true
	})
	return newS
}

func (s *FullCacheConcurrent) getAccountObject(addr common.Address) *accountObjectConcurrent {
	obj, ok := s.Accounts.Load(addr)
	if ok {
		return obj.(*accountObjectConcurrent)
	} else {
		return nil
	}
}

func (s *FullCacheConcurrent) setAccountObject(obj *accountObjectConcurrent) {
	s.Accounts.Store(obj.Address, obj)
}

// ------------------------------- Getter --------------------------------

// GetBalance 获取某个账户的余额
func (s *FullCacheConcurrent) GetBalance(addr common.Address) *big.Int {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetBalance()
	}
	return new(big.Int).SetInt64(0)
}

// GetNonce 获取nonce
func (s *FullCacheConcurrent) GetNonce(addr common.Address) uint64 {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetNonce()
	}
	return 0
}

// GetCodeHash 获取代码的hash值
func (s *FullCacheConcurrent) GetCodeHash(addr common.Address) common.Hash {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.CodeHash()
	}
	return common.Hash{}
}

// GetCode 获取智能合约的代码
func (s *FullCacheConcurrent) GetCode(addr common.Address) []byte {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.Code()
	}
	return nil
}

// GetCodeSize 获取code的大小
func (s *FullCacheConcurrent) GetCodeSize(addr common.Address) int {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		if stateObject.ByteCode != nil {
			return len(stateObject.ByteCode)
		} else {
			return 0
		}
	}
	return 0
}

// GetRefund ...
func (s *FullCacheConcurrent) GetRefund() uint64 {
	return 0
}

func (s *FullCacheConcurrent) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	return s.GetState(addr, key)
}

// GetState 和SetState 是用于保存合约执行时 存储的变量是否发生变化 evm对变量存储的改变消耗的gas是有区别的
func (s *FullCacheConcurrent) GetState(addr common.Address, key common.Hash) common.Hash {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		val, ok := stateObject.GetStorageState(key)
		if ok {
			return val
		}
		return common.Hash{}
	}
	return common.Hash{}
}

// GetTransientState gets transient storage for a given account.
func (s *FullCacheConcurrent) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.GetState(addr, key)
}

// Exist 检查账户是否存在
func (s *FullCacheConcurrent) Exist(addr common.Address) bool {
	return s.getAccountObject(addr) != nil
}

// Empty 是否是空账户
func (s *FullCacheConcurrent) Empty(addr common.Address) bool {
	so := s.getAccountObject(addr)
	return so == nil || so.Empty()
}

// ---------------------------------------- Setter -------------------------------------

func (s *FullCacheConcurrent) CreateAccount(addr common.Address) {
	if s.getAccountObject(addr) != nil {
		return
	}
	obj := newAccountObjectConcurrent(addr, accountData{})
	s.setAccountObject(obj)
}

func (s *FullCacheConcurrent) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
		return
	}
	// fmt.Println("SubBalance:", addr)
}

// AddBalance 增加某个账户的余额
func (s *FullCacheConcurrent) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
		return
	}
}

func (s *FullCacheConcurrent) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
		return
	}
}

func (s *FullCacheConcurrent) setBalancePrefetch(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
		return
	}
}

// SetNonce 设置nonce
func (s *FullCacheConcurrent) SetNonce(addr common.Address, nonce uint64) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
		return
	}
}

func (s *FullCacheConcurrent) setNoncePrefetch(addr common.Address, nonce uint64) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
		return
	}
}

// SetCode 设置智能合约的code
func (s *FullCacheConcurrent) SetCode(addr common.Address, code []byte) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
		return
	}
}

func (s *FullCacheConcurrent) setCodePrefetch(addr common.Address, code []byte) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.ByteCode = code
		return
	}
}

// This function only used in prefectching phase
func (s *FullCacheConcurrent) setCodeHashPrefetch(addr common.Address, codeHash common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.Data.CodeHash = codeHash
		return
	}
}

// AddRefund
func (s *FullCacheConcurrent) AddRefund(amount uint64) {
}

// SubRefund
func (s *FullCacheConcurrent) SubRefund(amount uint64) {
}

// SetState 设置变量的状态
func (s *FullCacheConcurrent) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		_, ok := stateObject.GetStorageState(key)
		if ok {
			// the value is present in the cache, so we need to record the change
			stateObject.SetStorageState(key, value)
		} else {
			// we write something that was not prefectched before, so we need to invalidate the cache
			// fmt.Println("SetState without slot:", addr, " ", key)
		}
		return
	}
	// fmt.Println("SetState without addr:", addr)
}

func (s *FullCacheConcurrent) setStatePrefetch(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetStorageState(key, value)
		return
	}
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *FullCacheConcurrent) SetTransientState(addr common.Address, key, value common.Hash) {
	s.SetState(addr, key, value)
}

// Suicide
func (s *FullCacheConcurrent) SelfDestruct(addr common.Address) {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	stateObject.IsAlive = false
	stateObject.Data.Balance = new(big.Int)
}

// HasSuicided ...
func (s *FullCacheConcurrent) HasSelfDestructed(addr common.Address) bool {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return false
	}
	return !stateObject.IsAlive
}

func (s *FullCacheConcurrent) Selfdestruct6780(addr common.Address) {
	s.SelfDestruct(addr)
}

func (s *FullCacheConcurrent) setIsAlivePrefetch(addr common.Address, isAlive bool) {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	stateObject.IsAlive = isAlive
}

// AddAddressToAccessList adds the given address to the access list
func (s *FullCacheConcurrent) AddAddressToAccessList(addr common.Address) {
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (s *FullCacheConcurrent) AddSlotToAccessList(addr common.Address, slot common.Hash) {
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (s *FullCacheConcurrent) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return true, true
}

// RevertToSnapshot ...
func (s *FullCacheConcurrent) RevertToSnapshot(revid int) {
}

// Snapshot ...
func (s *FullCacheConcurrent) Snapshot() int {
	return 0
}

// AddLog
func (s *FullCacheConcurrent) AddLog(log *types.Log) {
	// log.TxHash = s.thash
	// log.TxIndex = uint(s.txIndex)
	// log.Index = s.logSize
	// s.Logs[s.thash] = append(s.Logs[s.thash], log)
}

// AddPreimage
func (s *FullCacheConcurrent) AddPreimage(hash common.Hash, preimage []byte) {
}

func (s *FullCacheConcurrent) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
}

// AddressInAccessList returns true if the given address is in the access list.
func (s *FullCacheConcurrent) AddressInAccessList(addr common.Address) bool {
	return true
}

// SetTxContext sets the current transaction hash and index which are
// used when the EVM emits new state logs. It should be invoked before
// transaction execution.
func (s *FullCacheConcurrent) SetTxContext(thash common.Hash, ti int) {
	s.thash = thash
	s.txIndex = ti
}

func (s *FullCacheConcurrent) Prefetch(statedb vm.StateDB, rwSets []*accesslist.RWSet) {
	for _, rwSet := range rwSets {
		for addr, State := range rwSet.ReadSet {
			for hash := range State {
				s.prefetchSetter(addr, hash, statedb)
			}
		}
		for addr, State := range rwSet.WriteSet {
			for hash := range State {
				s.prefetchSetter(addr, hash, statedb)
			}
		}
	}
}

func (s *FullCacheConcurrent) prefetchSetter(addr common.Address, hash common.Hash, statedb vm.StateDB) {
	if s.prefectched.Contains(addr, hash) {
		return
	}
	s.prefectched.Add(addr, hash)

	s.CreateAccount(addr)
	switch hash {
	case accesslist.BALANCE:
		s.setBalancePrefetch(addr, statedb.GetBalance(addr))
	case accesslist.NONCE:
		s.setNoncePrefetch(addr, statedb.GetNonce(addr))
	case accesslist.CODEHASH:
		s.setCodeHashPrefetch(addr, statedb.GetCodeHash(addr))
	case accesslist.CODE:
		s.setCodePrefetch(addr, statedb.GetCode(addr))
	case accesslist.ALIVE:
		s.setIsAlivePrefetch(addr, statedb.Exist(addr))
	default:
		s.setStatePrefetch(addr, hash, statedb.GetState(addr, hash))
	}
}
