package tracer

import (
	"fmt"
	"interact/accesslist"
	cachestate "interact/cacheState"
	conflictgraph "interact/conflictGraph"
	"interact/core"
	"sort"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

// Execute 交易执行函数
func Execute(sdb vm.StateDB, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) error {
	// 使用range从txs中依次取出tx来执行
	for _, tx := range txs {
		// 交易执行
		// 这里对 cachestatedb 快照一下
		snapshot := sdb.Snapshot()
		_, err := ExecBasedOnRWSets(sdb, tx, header, chainCtx)
		if err != nil {
			if err == ErrFalsePredict {
				fmt.Println("False Predict List:", tx.Hash())
			}
			// 若出错则进行回滚
			sdb.RevertToSnapshot(snapshot)
			return err
		}
	}
	return nil
}

// 基于gopool并行执行execute函数
func ExecuteWithGopool(statedb *state.StateDB, predictAl []*accesslist.RWSet, txGroups [][]*conflictgraph.Vertex, txs []*types.Transaction, header *types.Header, chainCtx core.ChainContext) {
	// 	从groups中组装出交易来执行
	txsInGroup := make([][]*types.Transaction, len(txGroups))
	als := make([][]*accesslist.RWSet, len(txGroups))
	for i := 0; i < len(txGroups); i++ {
		sort.Slice(txGroups[i], func(j, k int) bool {
			return txGroups[i][j].TxId < txGroups[i][k].TxId
		})

		for j := 0; j < len(txGroups[i]); j++ {
			txsInGroup[i] = append(txsInGroup[i], txs[txGroups[i][j].TxId])
			als[i] = append(als[i], predictAl[txGroups[i][j].TxId])
		}
	}

	// 初始化一个gopool线程池,队列长度可设为分组组数
	// pool := gopool.NewGoPool(16, gopool.WithTaskQueueSize(len(txGroups)), gopool.WithMinWorkers(8))
	// defer pool.Release()

	// // 初始化一个缓存DB群
	cacheStateDb := make([]*cachestate.CacheState, len(txGroups))
	for i := 0; i < len(txGroups); i++ {
		cacheStateDb[i] = cachestate.NewStateDB()
		cacheStateDb[i].Prefetch(statedb, als[i])
	}
	// single group run
	fmt.Println(len(als[0]))
	ParallelExeFunc(txsInGroup[0], cacheStateDb[0], header, chainCtx)

	// // for j := 0; j < len(txGroups); j++ {
	// // 	taskNum := j
	// // 	pool.AddTask(func() (interface{}, error) {
	// // 		cacheStateDb[taskNum].Prefetch(statedb, als[taskNum])
	// // 		return nil, nil
	// // 	})
	// // }
	// // pool.Wait()
	// // fmt.Println("len of cacheStateDb:", len(cacheStateDb))
	// // fmt.Println("len of als:", len(als))

	// start := time.Now()
	// // 为每组交易创建一个任务
	// for j := 0; j < len(txGroups); j++ {
	// 	taskNum := j
	// 	pool.AddTask(func() (interface{}, error) {
	// 		return ParallelExeFunc(txsInGroup[taskNum], cacheStateDb[taskNum], header, chainCtx)
	// 	})
	// }
	// // 等待所有任务执行完毕
	// pool.Wait()

	// elapsed := time.Since(start)
	// fmt.Println("Parallel Execution Time:", elapsed)
	// // 将全部的cachestate合并到原有的statedb
	// for i := 0; i < len(cacheStateDb); i++ {
	// 	cacheStateDb[i].MerageState(statedb)
	// }

}

func ParallelExeFunc(txs []*types.Transaction, cacheStateDb *cachestate.CacheState, header *types.Header, chainCtx core.ChainContext) (interface{}, error) {
	// 使用cacheState执行交易
	err := Execute(cacheStateDb, txs, header, chainCtx)
	if err != nil {
		return nil, err
	}
	return nil, nil
}
