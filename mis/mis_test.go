package mis

import (
	"encoding/json"
	"fmt"
	conflictgraph "interact/conflictGraph"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func NewGraph() *conflictgraph.UndirectedGraph {
	G := conflictgraph.NewUndirectedGraph()
	for i := 0; i < 10; i++ {
		G.AddVertex(common.Hash{}, uint(i))
	}
	G.AddEdge(0, 1)
	G.AddEdge(0, 2)

	G.AddEdge(1, 2)
	G.AddEdge(1, 3)

	G.AddEdge(2, 3)

	G.AddEdge(3, 8)
	G.AddEdge(3, 4)

	G.AddEdge(4, 5)
	G.AddEdge(4, 7)

	G.AddEdge(5, 6)
	G.AddEdge(6, 7)

	G.AddEdge(8, 9)

	return G
}

func NewGraph2() *conflictgraph.UndirectedGraph {
	G := conflictgraph.NewUndirectedGraph()
	for i := 0; i < 6; i++ {
		G.AddVertex(common.Hash{}, uint(i))
	}
	G.AddEdge(0, 1)

	G.AddEdge(1, 2)
	G.AddEdge(1, 3)

	G.AddEdge(2, 4)
	G.AddEdge(2, 5)

	G.AddEdge(3, 4)
	G.AddEdge(3, 5)

	G.AddEdge(4, 5)

	return G
}

func TestSolveMIS(t *testing.T) {
	graphBytes, _ := os.ReadFile("../graph.json")
	var graph = &conflictgraph.UndirectedGraph{}
	json.Unmarshal(graphBytes, graph)

	// graph := NewGraph()
	// solution := NewSolution(graph)
	// solution.Solve()
	// vertices := solution.IndependentSet.ToSlice()
	// for _, v := range vertices {
	// 	t.Log(v.(uint))
	// }
	for {
		MisSolution := NewSolution(graph)
		MisSolution.Solve()
		ansSlice := MisSolution.IndependentSet.ToSlice()
		t.Log(len(ansSlice))

		for _, v := range graph.Vertices {
			v.IsDeleted = false
			v.Degree = uint(len(graph.AdjacencyMap[v.TxId]))
		}
		if len(ansSlice) <= 3 {
			edgeCount := 0
			for id := range graph.Vertices {
				edgeCount += len(graph.AdjacencyMap[id])
			}
			edgeCount /= 2
			fmt.Println("Node Cound:", len(graph.Vertices))
			fmt.Println("Edge Count:", edgeCount)
		}
		for _, v := range ansSlice {
			graph.Vertices[v.(uint)].IsDeleted = true
		}
		graph = graph.CopyGraphWithDeletion()
		if len(graph.Vertices) == 0 {
			break
		}
	}
}
