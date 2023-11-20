package utils

import (
	conflictgraph "interact/conflictGraph"
	"interact/mis"
)

// solveMISInTurn an approximation algorithm to solve MIS problem
func solveMISInTurn(undiConfGraph *conflictgraph.UndirectedGraph) [][]uint {
	ans := make([][]uint, 0)
	for {
		MisSolution := mis.NewSolution(undiConfGraph)
		MisSolution.Solve()
		ansSlice := MisSolution.IndependentSet.ToSlice()
		ansSliceUint := make([]uint, len(ansSlice))
		for i, v := range ansSlice {
			ansSliceUint[i] = v.(uint)
		}
		ans = append(ans, ansSliceUint)
		for _, v := range undiConfGraph.Vertices {
			v.IsDeleted = false
			v.Degree = uint(len(undiConfGraph.AdjacencyMap[v.TxId]))
		}
		for _, v := range ansSlice {
			undiConfGraph.Vertices[v.(uint)].IsDeleted = true
		}
		undiConfGraph = undiConfGraph.CopyGraphWithDeletion()
		if len(undiConfGraph.Vertices) == 0 {
			break
		}
	}
	return ans
}
