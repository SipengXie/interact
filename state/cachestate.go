package state

import (
	"fmt"
	"interact/accesslist"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// CacheState for APEX must support snapshot and revert
// We should determine whether the group of transaction is conflict-free or not
// For APEX, the group of transaction is conflicted, so each group own its own cache state

// For APEX+, the group of transaction is conflict-free?
// For APEX+, this may be much more difficult for snapshot and revert? We need execution phase and commit phase, where each tx gets its own cache state.

// CacheState for APEX, we could just leverage stateDB, but prefetch the data?
// The stateDB instance used for each thread should be finally merged into the main stateDB at the commit phase

// CacheState for APEX+, we could implement a new KVS, without the need to support snapshot and revert.
// Because the CacheState is for each transaction, and the commit phase and execution phase happen in turn.
// Or use a overrall KVS, but we need to support thread safe snapshot and revert,
// and under this circumstance, we don't need to force commit phase and execution phase to happen in turn.

type revision struct {
	id           int
	journalIndex int
}

// CacheState for APEX & APEX+
type CacheState struct {
	Accounts map[common.Address]*accountObject `json:"accounts,omitempty"`

	Logs           map[common.Hash][]*types.Log `json:"logs,omitempty"`
	thash          common.Hash
	txIndex        int
	logSize        uint
	StateJudge     bool
	prefetching    bool
	prefectched    accesslist.ALTuple
	Journal        *journal `json:"journal,omitempty"`
	ValidRevisions []revision
	NextRevisionId int
}

type CacheStateList []*CacheState

func NewCacheState() *CacheState {
	return &CacheState{
		Accounts:    make(map[common.Address]*accountObject),
		Journal:     newJournal(),
		Logs:        make(map[common.Hash][]*types.Log),
		StateJudge:  true,
		prefetching: false,
		prefectched: make(accesslist.ALTuple),
	}
}

func (s *CacheState) getAccountObject(addr common.Address) *accountObject {
	obj, ok := s.Accounts[addr]
	if ok {
		return obj
	} else {
		return nil
	}
}

func (s *CacheState) setAccountObject(obj *accountObject) {
	s.Accounts[obj.Address] = obj
}

// ------------------------------- Getter --------------------------------

// GetBalance 获取某个账户的余额
func (s *CacheState) GetBalance(addr common.Address) *big.Int {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetBalance()
	}
	s.StateJudge = false
	return new(big.Int).SetInt64(0)
}

// GetNonce 获取nonce
func (s *CacheState) GetNonce(addr common.Address) uint64 {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetNonce()
	}
	s.StateJudge = false
	return 0
}

// GetCodeHash 获取代码的hash值
func (s *CacheState) GetCodeHash(addr common.Address) common.Hash {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.CodeHash()
	}
	s.StateJudge = false
	return common.Hash{}
}

// GetCode 获取智能合约的代码
func (s *CacheState) GetCode(addr common.Address) []byte {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.Code()
	}
	s.StateJudge = false
	return nil
}

// GetCodeSize 获取code的大小
func (s *CacheState) GetCodeSize(addr common.Address) int {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		if stateObject.ByteCode != nil {
			return len(stateObject.ByteCode)
		} else {
			return 0
		}
	}
	s.StateJudge = false
	return 0
}

// GetRefund ...
func (s *CacheState) GetRefund() uint64 {
	return 0
}

func (s *CacheState) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	return s.GetState(addr, key)
}

// GetState 和SetState 是用于保存合约执行时 存储的变量是否发生变化 evm对变量存储的改变消耗的gas是有区别的
func (s *CacheState) GetState(addr common.Address, key common.Hash) common.Hash {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		val, ok := stateObject.GetStorageState(key)
		if ok {
			return val
		}
		s.StateJudge = false
		return common.Hash{}
	}
	s.StateJudge = false
	return common.Hash{}
}

// GetTransientState gets transient storage for a given account.
func (s *CacheState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.GetState(addr, key)
}

// Exist 检查账户是否存在
func (s *CacheState) Exist(addr common.Address) bool {
	return s.getAccountObject(addr) != nil
}

// Empty 是否是空账户
func (s *CacheState) Empty(addr common.Address) bool {
	so := s.getAccountObject(addr)
	return so == nil || so.Empty()
}

// ---------------------------------------- Setter -------------------------------------

func (s *CacheState) CreateAccount(addr common.Address) {
	if s.getAccountObject(addr) != nil {
		return
	}
	if !s.prefetching {
		s.Journal.append(createObjectChange{&addr})
	}
	obj := newAccountObject(addr, accountData{})
	s.setAccountObject(obj)
}

func (s *CacheState) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		if !s.prefetching {
			s.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		}
		stateObject.SubBalance(amount)
		return
	}
	// fmt.Println("SubBalance:", addr)
	s.StateJudge = false
}

// AddBalance 增加某个账户的余额
func (s *CacheState) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		if !s.prefetching {
			s.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		}
		stateObject.AddBalance(amount)
		return
	}
	s.StateJudge = false
}

func (s *CacheState) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		s.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		stateObject.SetBalance(amount)
		return
	}
	s.StateJudge = false
}

func (s *CacheState) setBalancePrefetch(addr common.Address, amount *big.Int) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
		return
	}
}

// SetNonce 设置nonce
func (s *CacheState) SetNonce(addr common.Address, nonce uint64) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		s.Journal.append(nonceChange{&addr, stateObject.Data.Nonce})
		stateObject.SetNonce(nonce)
		return
	}
	s.StateJudge = false
}

func (s *CacheState) setNoncePrefetch(addr common.Address, nonce uint64) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
		return
	}
}

// SetCode 设置智能合约的code
func (s *CacheState) SetCode(addr common.Address, code []byte) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		if !s.prefetching {
			s.Journal.append(codeChange{&addr, stateObject.ByteCode, stateObject.Data.CodeHash.Bytes()})
		}
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
		return
	}
	s.StateJudge = false
}

func (s *CacheState) setCodePrefetch(addr common.Address, code []byte) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.ByteCode = code
		return
	}
}

// This function only used in prefectching phase
func (s *CacheState) setCodeHashPrefetch(addr common.Address, codeHash common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.Data.CodeHash = codeHash
		return
	}
}

// AddRefund
func (s *CacheState) AddRefund(amount uint64) {
}

// SubRefund
func (s *CacheState) SubRefund(amount uint64) {
}

// SetState 设置变量的状态
func (s *CacheState) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		val, ok := stateObject.GetStorageState(key)
		if ok {
			// the value is present in the cache, so we need to record the change
			s.Journal.append(storageChange{&addr, key, val})
			stateObject.SetStorageState(key, value)
		} else {
			// we write something that was not prefectched before, so we need to invalidate the cache
			// fmt.Println("SetState without slot:", addr, " ", key)
			s.StateJudge = false
		}
		return
	}
	// fmt.Println("SetState without addr:", addr)
	s.StateJudge = false
}

func (s *CacheState) setStatePrefetch(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := s.getAccountObject(addr)
	if stateObject != nil {
		stateObject.SetStorageState(key, value)
		return
	}
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *CacheState) SetTransientState(addr common.Address, key, value common.Hash) {
	s.SetState(addr, key, value)
}

// Suicide
func (s *CacheState) SelfDestruct(addr common.Address) {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	s.Journal.append(selfDestructChange{
		account:     &addr,
		prev:        stateObject.IsAlive,
		prevbalance: stateObject.Data.Balance,
	})
	stateObject.IsAlive = false
	stateObject.Data.Balance = new(big.Int)
}

// HasSuicided ...
func (s *CacheState) HasSelfDestructed(addr common.Address) bool {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return false
	}
	return !stateObject.IsAlive
}

func (s *CacheState) Selfdestruct6780(addr common.Address) {
	s.SelfDestruct(addr)
}

func (s *CacheState) setIsAlivePrefetch(addr common.Address, isAlive bool) {
	stateObject := s.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	stateObject.IsAlive = isAlive
}

// AddAddressToAccessList adds the given address to the access list
func (s *CacheState) AddAddressToAccessList(addr common.Address) {
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (s *CacheState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (s *CacheState) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return true, true
}

// RevertToSnapshot ...
func (s *CacheState) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.ValidRevisions), func(i int) bool {
		return s.ValidRevisions[i].id >= revid
	})
	if idx == len(s.ValidRevisions) || s.ValidRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.ValidRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	s.Journal.revert(s, snapshot)
	s.ValidRevisions = s.ValidRevisions[:idx]
}

// Snapshot ...
func (s *CacheState) Snapshot() int {
	id := s.NextRevisionId
	s.NextRevisionId++
	s.ValidRevisions = append(s.ValidRevisions, revision{id, s.Journal.length()})
	return id
}

// AddLog
func (s *CacheState) AddLog(log *types.Log) {
	log.TxHash = s.thash
	log.TxIndex = uint(s.txIndex)
	log.Index = s.logSize
	s.Logs[s.thash] = append(s.Logs[s.thash], log)
}

// AddPreimage
func (s *CacheState) AddPreimage(hash common.Hash, preimage []byte) {
}

func (s *CacheState) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
}

// AddressInAccessList returns true if the given address is in the access list.
func (s *CacheState) AddressInAccessList(addr common.Address) bool {
	return true
}

// SetTxContext sets the current transaction hash and index which are
// used when the EVM emits new state logs. It should be invoked before
// transaction execution.
func (s *CacheState) SetTxContext(thash common.Hash, ti int) {
	s.thash = thash
	s.txIndex = ti
}

// 若存在无法读取或写入的地址orslot，将stateJudge置为false
func (s *CacheState) SetTxStateErr(thash common.Hash, ti int) {
	s.thash = thash
	s.txIndex = ti
	s.StateJudge = false
}

func (s *CacheState) Prefetch(statedb vm.StateDB, rwSets []*accesslist.RWSet) {
	// 预取时置prefetching为true
	s.prefetching = true
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
	// fmt.Println("After prefetching, the cache state judge is:", s.StateJudge)
	// 预取结束后置prefetching为false
	s.prefetching = false
}

func (s *CacheState) prefetchSetter(addr common.Address, hash common.Hash, statedb vm.StateDB) {
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

// ! we can use write set to optimize the merge process
func (s *CacheState) MergeState(statedb StateInterface) {
	for addr := range s.Journal.dirties {
		aoj := s.getAccountObject(addr)
		statedb.SetBalance(addr, aoj.GetBalance())
		statedb.SetNonce(addr, aoj.GetNonce())
		statedb.SetCode(addr, aoj.Code())
		for slot, value := range aoj.CacheStorage {
			statedb.SetState(addr, slot, value)
		}
	}
}
