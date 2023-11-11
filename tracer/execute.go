package tracer

import (
	"interact/accesslist"
	cachestate "interact/cacheState"
	conflictgraph "interact/conflictGraph"
	"interact/core"

	"github.com/devchat-ai/gopool"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

// Execute 交易执行函数
func Execute(cacheStateDb *cachestate.CacheState, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) error {
	// 使用range从txs中依次取出tx来执行
	for _, tx := range txs {
		// 交易执行
		// 这里对 cachestatedb 快照一下
		snapshot := cacheStateDb.Snapshot()
		_, err := ExecBasedOnRWSets(cacheStateDb, tx, header, chainCtx)
		if err != nil {
			// 若出错则进行回滚
			cacheStateDb.RevertToSnapshot(snapshot)
			return err
		}
		// TODO：后续传predict进行对比
	}
	return nil
}

// 基于gopool并行执行execute函数
func ExecuteWithGopool(statedb *state.StateDB, predictAl []*accesslist.RWSet, txGroups [][]*conflictgraph.Vertex, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) {
	// 	从groups中组装出交易来执行
	txsInGroup := make([][]*types.Transaction, len(txGroups))
	als := make([][]*accesslist.RWSet, len(txGroups))
	for i := 0; i < len(txGroups); i++ {
		for j := 0; j < len(txGroups[i]); j++ {
			txsInGroup[i][j] = txs[txGroups[i][j].TxId]
			als[i][j] = predictAl[txGroups[i][j].TxId]
		}
	}

	// 初始化一个gopool线程池,队列长度可设为分组组数
	pool := gopool.NewGoPool(16, gopool.WithTaskQueueSize(len(txGroups)), gopool.WithMinWorkers(8))
	defer pool.Release()

	// 初始化一个缓存DB群
	cacheStateDb := make([]*cachestate.CacheState, len(txGroups))
	// ！忘了会不会自动调用new了
	for i := 0; i < len(cacheStateDb); i++ {
		cacheStateDb[i] = cachestate.NewStateDB()
	}
	// 为每组交易创建一个任务
	for i := 0; i < len(txGroups); i++ {
		pool.AddTask(func() (interface{}, error) {
			// 使用cacheStatedb
			cacheStateDb[i].Prefetch(statedb, als[i])
			// 使用cacheState执行交易
			err := Execute(cacheStateDb[i], txsInGroup[i], header, chainCtx)
			if err != nil {
				return nil, err
			}

			return nil, nil
		})
	}
	// 等待所有任务执行完毕
	pool.Wait()

	// 将全部的cachestate合并到原有的statedb
	for i := 0; i < len(cacheStateDb); i++ {
		cacheStateDb[i].MerageState(statedb)
	}
}

// 预取数据填入cacheStateDb
// for _, al := range als[i] {
// 	mAl := al.ToMarshal()
// 	// TODO:将全部取改为只取al内的
// 	// 从读写集中取出地址并找到数据记录进cacheState中
// 	for addr, _ := range mAl.ReadSet {
// 		soj := statedb.GetOrNewStateObject(addr)
// 		acData := cachestate.NewAccountData(soj.Nonce(), soj.Balance(), soj.Root(), soj.CodeHash(), soj.Code())
// 		// state
// 		aoj := cachestate.NewAccountObject(addr, acData)
// 		cacheStateDb[i].Accounts[addr] = aoj
// 	}
// 	for addr, _ := range mAl.WriteSet {
// 		soj := statedb.GetOrNewStateObject(addr)
// 		acData := cachestate.NewAccountData(soj.Nonce(), soj.Balance(), soj.Root(), soj.CodeHash(), soj.Code())
// 		aoj := cachestate.NewAccountObject(addr, acData)
// 		cacheStateDb[i].Accounts[addr] = aoj
// 	}
// }
