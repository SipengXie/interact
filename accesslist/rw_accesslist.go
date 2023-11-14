package accesslist

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/common"
)

var (
	CODE     = common.Hash(sha256.Sum256([]byte("code")))
	CODEHASH = common.Hash(sha256.Sum256([]byte("codeHash")))
	BALANCE  = common.Hash(sha256.Sum256([]byte("balance")))
	NONCE    = common.Hash(sha256.Sum256([]byte("nonce")))
	ALIVE    = common.Hash(sha256.Sum256([]byte("alive")))
)

type State map[common.Hash]struct{}

type ALTuple map[common.Address]State

func (tuple ALTuple) Add(addr common.Address, hash common.Hash) {
	if _, ok := tuple[addr]; !ok {
		tuple[addr] = make(State)
	}
	tuple[addr][hash] = struct{}{}
}

func (tuple ALTuple) Contains(addr common.Address, hash common.Hash) bool {
	_, ok := tuple[addr][hash]
	return ok
}

type RWSet struct {
	ReadSet  ALTuple
	WriteSet ALTuple
}

func NewRWAccessLists() *RWSet {
	return &RWSet{
		ReadSet:  make(ALTuple),
		WriteSet: make(ALTuple),
	}
}

func (RWSets RWSet) AddReadSet(addr common.Address, hash common.Hash) {
	RWSets.ReadSet.Add(addr, hash)
}

func (RWSets RWSet) AddWriteSet(addr common.Address, hash common.Hash) {
	RWSets.WriteSet.Add(addr, hash)
}

func (RWSets RWSet) HasConflict(other RWSet) bool {
	for addr, state := range RWSets.ReadSet {
		for hash := range state {
			if other.WriteSet.Contains(addr, hash) {
				return true
			}
		}
	}
	for addr, state := range RWSets.WriteSet {
		for hash := range state {
			if other.WriteSet.Contains(addr, hash) {
				return true
			}
			if other.ReadSet.Contains(addr, hash) {
				return true
			}
		}
	}
	return false
}

func (RWSets RWSet) Equal(other RWSet) bool {
	if len(RWSets.ReadSet) != len(other.ReadSet) {
		return false
	}
	if len(RWSets.WriteSet) != len(other.WriteSet) {
		return false
	}

	for addr, state := range RWSets.ReadSet {
		for hash := range state {
			if !other.ReadSet.Contains(addr, hash) {
				return false
			}
		}
	}

	for addr, state := range RWSets.WriteSet {
		for hash := range state {
			if !other.WriteSet.Contains(addr, hash) {
				return false
			}
		}
	}

	return true
}

func decodeHash(hash common.Hash) string {
	switch hash {
	case CODE:
		return "code"
	case BALANCE:
		return "balance"
	case ALIVE:
		return "alive"
	case CODEHASH:
		return "codeHash"
	case NONCE:
		return "nonce"
	default:
		return hash.Hex()
	}
}

func encodeHash(str string) common.Hash {
	switch str {
	case "code":
		return CODE
	case "balance":
		return BALANCE
	case "alive":
		return ALIVE
	case "codeHash":
		return CODEHASH
	case "nonce":
		return NONCE
	default:
		return common.HexToHash(str)
	}
}

func (RWSets RWSet) ToJsonStruct() RWSetJson {
	readAL := make(map[common.Address][]string)
	writeAL := make(map[common.Address][]string)

	for addr, state := range RWSets.ReadSet {
		for hash := range state {
			readAL[addr] = append(readAL[addr], decodeHash(hash))
		}
	}

	for addr, state := range RWSets.WriteSet {
		for hash := range state {
			writeAL[addr] = append(writeAL[addr], decodeHash(hash))
		}
	}

	return RWSetJson{
		ReadSet:  readAL,
		WriteSet: writeAL,
	}
}

type RWSetJson struct {
	ReadSet  map[common.Address][]string `json:"readAL"`
	WriteSet map[common.Address][]string `json:"writeAL"`
}
