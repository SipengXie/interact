package cachestate

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type accountData struct {
	Nonce    uint64      `json:"nonce,omitempty"`
	Balance  *big.Int    `json:"balance,omitempty"`
	Root     common.Hash `json:"root,omitempty"` // MPT root of the storage trie
	CodeHash common.Hash `json:"code_hash,omitempty"`
}

type accountObject struct {
	Address      common.Address              `json:"address,omitempty"`
	ByteCode     []byte                      `json:"byte_code,omitempty"`
	Data         accountData                 `json:"data,omitempty"`
	CacheStorage map[common.Hash]common.Hash `json:"cache_storage,omitempty"` // 用于缓存存储的变量
	IsAlive      bool                        `json:"is_alive,omitempty"`
}

func newAccountObject(address common.Address, data accountData) *accountObject {
	if data.Balance == nil {
		data.Balance = new(big.Int)
	}
	if (data.CodeHash == common.Hash{}) {
		data.CodeHash = types.EmptyCodeHash
	}
	return &accountObject{
		Address:      address,
		Data:         data,
		CacheStorage: make(map[common.Hash]common.Hash),
		IsAlive:      true,
	}
}

func (object *accountObject) GetBalance() *big.Int {
	return object.Data.Balance
}

func (object *accountObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	object.Data.Balance = new(big.Int).Sub(object.Data.Balance, amount)
}

func (object *accountObject) AddBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	object.Data.Balance = new(big.Int).Add(object.Data.Balance, amount)
}

func (object *accountObject) SetBalance(amount *big.Int) {
	object.Data.Balance = amount
}

func (object *accountObject) GetNonce() uint64 {
	return object.Data.Nonce
}

func (object *accountObject) SetNonce(nonce uint64) {
	object.Data.Nonce = nonce
}

func (object *accountObject) CodeHash() common.Hash {
	return object.Data.CodeHash
}

func (object *accountObject) Code() []byte {
	return object.ByteCode
}

func (object *accountObject) SetCode(codeHash common.Hash, code []byte) {
	object.Data.CodeHash = codeHash
	object.ByteCode = code
}

func (object *accountObject) GetStorageState(key common.Hash) (common.Hash, bool) {
	value, exist := object.CacheStorage[key]
	if exist {
		// fmt.Println("exist cache ", " key: ", key, " value: ", value)
		return value, true
	}
	return common.Hash{}, false
}

func (object *accountObject) SetStorageState(key, value common.Hash) {
	object.CacheStorage[key] = value
}

func (object *accountObject) Empty() bool {
	return object.Data.Nonce == 0 && object.Data.Balance.Sign() == 0 && (object.Data.CodeHash == types.EmptyCodeHash)
}
