package cachestate

import (
	"fmt"
	"interact/accesslist"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
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

	Logs    map[common.Hash][]*types.Log `json:"logs,omitempty"`
	thash   common.Hash
	txIndex int
	logSize uint

	Journal        *journal `json:"journal,omitempty"`
	ValidRevisions []revision
	NextRevisionId int
}

func NewStateDB() *CacheState {
	return &CacheState{
		Accounts: make(map[common.Address]*accountObject),
		Journal:  newJournal(),
		Logs:     make(map[common.Hash][]*types.Log),
	}
}

func (accSt *CacheState) getAccountObject(addr common.Address) *accountObject {
	obj, ok := accSt.Accounts[addr]
	if ok {
		return obj
	} else {
		return nil
	}
}

func (accSt *CacheState) setAccountObject(obj *accountObject) {
	accSt.Accounts[obj.Address] = obj
}

func (accSt *CacheState) getOrsetAccountObject(addr common.Address) *accountObject {
	get := accSt.getAccountObject(addr)
	if get != nil {
		return get
	}
	set := newAccountObject(addr, accountData{})
	accSt.setAccountObject(set)
	return set
}

// CreateAccount 创建账户接口
func (accSt *CacheState) CreateAccount(addr common.Address) {

	if accSt.getAccountObject(addr) != nil {
		// 我们忽略addr冲突的情况
		return
	}
	accSt.Journal.append(createObjectChange{&addr})
	obj := newAccountObject(addr, accountData{})
	accSt.setAccountObject(obj)
}

// SubBalance 减去某个账户的余额
func (accSt *CacheState) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		accSt.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		stateObject.SubBalance(amount)
	}
}

// AddBalance 增加某个账户的余额
func (accSt *CacheState) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		accSt.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		stateObject.AddBalance(amount)
	}
}

func (accSt *CacheState) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		accSt.Journal.append(balanceChange{&addr, stateObject.Data.Balance})
		stateObject.SetBalance(amount)
	}
}

// GetBalance 获取某个账户的余额
func (accSt *CacheState) GetBalance(addr common.Address) *big.Int {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetBalance()
	}
	return new(big.Int).SetInt64(0)
}

// GetNonce 获取nonce
func (accSt *CacheState) GetNonce(addr common.Address) uint64 {
	stateObject := accSt.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetNonce()
	}
	return 0
}

// SetNonce 设置nonce
func (accSt *CacheState) SetNonce(addr common.Address, nonce uint64) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		accSt.Journal.append(nonceChange{&addr, stateObject.Data.Nonce})
		stateObject.SetNonce(nonce)
	}
}

// GetCodeHash 获取代码的hash值
func (accSt *CacheState) GetCodeHash(addr common.Address) common.Hash {
	stateObject := accSt.getAccountObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return stateObject.CodeHash()
}

// GetCode 获取智能合约的代码
func (accSt *CacheState) GetCode(addr common.Address) []byte {
	stateObject := accSt.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.Code()
	}
	return nil
}

// SetCode 设置智能合约的code
func (accSt *CacheState) SetCode(addr common.Address, code []byte) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		accSt.Journal.append(codeChange{&addr, stateObject.ByteCode, stateObject.Data.CodeHash.Bytes()})
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

// GetCodeSize 获取code的大小
func (accSt *CacheState) GetCodeSize(addr common.Address) int {
	stateObject := accSt.getAccountObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.ByteCode != nil {
		return len(stateObject.ByteCode)
	}
	return 0
}

// AddRefund
func (accSt *CacheState) AddRefund(amount uint64) {
}

// SubRefund
func (accSt *CacheState) SubRefund(amount uint64) {
}

// GetRefund ...
func (accSt *CacheState) GetRefund() uint64 {
	return 0
}

func (accSt *CacheState) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	return accSt.GetState(addr, key)
}

// GetState 和SetState 是用于保存合约执行时 存储的变量是否发生变化 evm对变量存储的改变消耗的gas是有区别的
func (accSt *CacheState) GetState(addr common.Address, key common.Hash) common.Hash {
	stateObject := accSt.getAccountObject(addr)
	if stateObject != nil {
		return stateObject.GetStorageState(key)
	}
	return common.Hash{}
}

// SetState 设置变量的状态
func (accSt *CacheState) SetState(addr common.Address, key common.Hash, value common.Hash) {
	stateObject := accSt.getOrsetAccountObject(addr)
	if stateObject != nil {
		// fmt.Printf("SetState key: %x value: %s", key, new(big.Int).SetBytes(value[:]).String())
		accSt.Journal.append(storageChange{&addr, key, stateObject.GetStorageState(key)})
		stateObject.SetStorageState(key, value)
	}
}

// GetTransientState gets transient storage for a given account.
func (accSt *CacheState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return accSt.GetState(addr, key)
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (accSt *CacheState) SetTransientState(addr common.Address, key, value common.Hash) {
	accSt.SetState(addr, key, value)
}

// Suicide
func (accSt *CacheState) SelfDestruct(addr common.Address) {
	stateObject := accSt.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	accSt.Journal.append(selfDestructChange{
		account:     &addr,
		prev:        stateObject.IsAlive,
		prevbalance: stateObject.Data.Balance,
	})
	stateObject.IsAlive = false
	stateObject.Data.Balance = new(big.Int)
}

// HasSuicided ...
func (accSt *CacheState) HasSelfDestructed(addr common.Address) bool {
	stateObject := accSt.getAccountObject(addr)
	if stateObject == nil {
		return false
	}
	return !stateObject.IsAlive
}

func (accSt *CacheState) Selfdestruct6780(addr common.Address) {
	accSt.SelfDestruct(addr)
}

func (accSt *CacheState) SetIsAlive(addr common.Address, isAlive bool) {
	stateObject := accSt.getAccountObject(addr)
	if stateObject == nil {
		return
	}
	stateObject.IsAlive = isAlive
}

// Exist 检查账户是否存在
func (accSt *CacheState) Exist(addr common.Address) bool {
	return accSt.getAccountObject(addr) != nil
}

// Empty 是否是空账户
func (accSt *CacheState) Empty(addr common.Address) bool {
	so := accSt.getAccountObject(addr)
	return so == nil || so.Empty()
}

// AddAddressToAccessList adds the given address to the access list
func (accSt *CacheState) AddAddressToAccessList(addr common.Address) {
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (accSt *CacheState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (accSt *CacheState) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return false, false
}

// RevertToSnapshot ...
func (accSt *CacheState) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(accSt.ValidRevisions), func(i int) bool {
		return accSt.ValidRevisions[i].id >= revid
	})
	if idx == len(accSt.ValidRevisions) || accSt.ValidRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := accSt.ValidRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	accSt.Journal.revert(accSt, snapshot)
	accSt.ValidRevisions = accSt.ValidRevisions[:idx]
}

// Snapshot ...
func (accSt *CacheState) Snapshot() int {
	id := accSt.NextRevisionId
	accSt.NextRevisionId++
	accSt.ValidRevisions = append(accSt.ValidRevisions, revision{id, accSt.Journal.length()})
	return id
}

// AddLog
func (accSt *CacheState) AddLog(log *types.Log) {
	log.TxHash = accSt.thash
	log.TxIndex = uint(accSt.txIndex)
	log.Index = accSt.logSize
	accSt.Logs[accSt.thash] = append(accSt.Logs[accSt.thash], log)
}

// AddPreimage
func (accSt *CacheState) AddPreimage(hash common.Hash, preimage []byte) {
}

func (accSt *CacheState) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address, precompiles []common.Address, list types.AccessList) {
}

// AddressInAccessList returns true if the given address is in the access list.
func (accSt *CacheState) AddressInAccessList(addr common.Address) bool {
	return false
}

// SetTxContext sets the current transaction hash and index which are
// used when the EVM emits new state logs. It should be invoked before
// transaction execution.
func (s *CacheState) SetTxContext(thash common.Hash, ti int) {
	s.thash = thash
	s.txIndex = ti
}

func (s *CacheState) Prefetch(statedb *state.StateDB, rwSets []*accesslist.RW_AccessLists) {
	for _, rwSet := range rwSets {
		for element := range rwSet.ReadAL {
			prefetchSetter(element, s, statedb)
		}
		for element := range rwSet.WriteAL {
			prefetchSetter(element, s, statedb)
		}
	}
}

func prefetchSetter(element accesslist.Byte52, s *CacheState, statedb *state.StateDB) {
	addr := common.BytesToAddress(element[:20])
	hash := common.BytesToHash(element[20:])
	s.CreateAccount(addr)
	switch hash {
	case accesslist.BALANCE:
		s.SetBalance(addr, statedb.GetBalance(addr))
	case accesslist.NONCE:
		s.SetNonce(addr, statedb.GetNonce(addr))
	case accesslist.CODEHASH:
		s.SetCode(addr, statedb.GetCode(addr)) // 这里拆得不够细
	case accesslist.CODE:
		s.SetCode(addr, statedb.GetCode(addr))
	case accesslist.ALIVE:
		s.SetIsAlive(addr, statedb.Exist(addr))
	default:
		s.SetState(addr, hash, statedb.GetState(addr, hash))
	}
}
