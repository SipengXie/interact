package accesslist

import (
	cmap "github.com/orcaman/concurrent-map/v2"
)

// IndexTuple is a highly written map
// so we need use concurrent map[]

type ReserveSet struct {
	innerMap cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, uint]]
}

func NewReserveSet() *ReserveSet {
	return &ReserveSet{
		innerMap: cmap.New[cmap.ConcurrentMap[string, uint]](),
	}
}

func (rs *ReserveSet) get(addrStr, hashStr string) (uint, bool) {
	innerMap, ok := rs.innerMap.Get(addrStr)
	if !ok {
		return 0, false
	}
	return innerMap.Get(hashStr)
}

func (rs *ReserveSet) set(addrStr, hashStr string, Tid uint) {
	innerMap, ok := rs.innerMap.Get(addrStr)
	if !ok {
		innerMap = cmap.New[uint]()
		rs.innerMap.Set(addrStr, innerMap)
	}
	innerMap.Set(hashStr, Tid)
}

func (rs *ReserveSet) Reserve(set ALTuple, Tid uint) {
	for addr, state := range set {
		for hash := range state {
			addrStr := addr.Hex()
			hashStr := hash.Hex()
			reservId, exist := rs.get(addrStr, hashStr)
			if !exist || Tid < reservId {
				rs.set(addrStr, hashStr, Tid)
			}
		}
	}
}

func (rs *ReserveSet) HasConflict(Tid uint, set ALTuple) bool {
	for addr, state := range set {
		for hash := range state {
			addrStr := addr.Hex()
			hashStr := hash.Hex()
			reservId, exist := rs.get(addrStr, hashStr)
			if exist && reservId < Tid {
				return true
			}
		}
	}
	return false
}
