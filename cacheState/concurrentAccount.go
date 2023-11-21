package cachestate

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type accountObjectConcurrent struct {
	Address      common.Address `json:"address,omitempty"`
	ByteCode     []byte         `json:"byte_code,omitempty"`
	Data         accountData    `json:"data,omitempty"`
	CacheStorage sync.Map       `json:"cache_storage,omitempty"` // 用于缓存存储的变量
	IsAlive      bool           `json:"is_alive,omitempty"`
}

func newAccountObjectConcurrent(address common.Address, data accountData) *accountObjectConcurrent {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if (data.CodeHash == common.Hash{}) {
		data.CodeHash = types.EmptyCodeHash
	}
	return &accountObjectConcurrent{
		Address:      address,
		Data:         data,
		CacheStorage: sync.Map{},
		IsAlive:      true,
	}
}

func (object *accountObjectConcurrent) GetBalance() *big.Int {
	return object.Data.Balance
}

func (object *accountObjectConcurrent) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	object.Data.Balance = new(big.Int).Sub(object.Data.Balance, amount)
}

func (object *accountObjectConcurrent) AddBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	object.Data.Balance = new(big.Int).Add(object.Data.Balance, amount)
}

func (object *accountObjectConcurrent) SetBalance(amount *big.Int) {
	object.Data.Balance = amount
}

func (object *accountObjectConcurrent) GetNonce() uint64 {
	return object.Data.Nonce
}

func (object *accountObjectConcurrent) SetNonce(nonce uint64) {
	object.Data.Nonce = nonce
}

func (object *accountObjectConcurrent) CodeHash() common.Hash {
	return object.Data.CodeHash
}

func (object *accountObjectConcurrent) Code() []byte {
	return object.ByteCode
}

func (object *accountObjectConcurrent) SetCode(codeHash common.Hash, code []byte) {
	object.Data.CodeHash = codeHash
	object.ByteCode = code
}

func (object *accountObjectConcurrent) GetStorageState(key common.Hash) (common.Hash, bool) {
	value, exist := object.CacheStorage.Load(key)
	if exist {
		// fmt.Println("exist cache ", " key: ", key, " value: ", value)
		return value.(common.Hash), true
	}
	return common.Hash{}, false
}

func (object *accountObjectConcurrent) SetStorageState(key, value common.Hash) {
	object.CacheStorage.Store(key, value)
}

func (object *accountObjectConcurrent) Empty() bool {
	return object.Data.Nonce == 0 && object.Data.Balance.Sign() == 0 && (object.Data.CodeHash == types.EmptyCodeHash)
}
