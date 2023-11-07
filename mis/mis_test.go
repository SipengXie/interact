package mis

import (
	conflictgraph "interact/conflictGraph"
	"testing"
)

func NewGraph() *conflictgraph.UndirectedGraph {
	G := conflictgraph.NewUndirectedGraph()
	vertices := make([]*conflictgraph.Vertex, 0)
	for i := 0; i < 10; i++ {
		vertices = append(vertices, &conflictgraph.Vertex{
			TxId:      i,
			IsDeleted: false,
			Degree:    0,
		})
		G.AddVertex(vertices[i])
	}
	G.AddEdge(vertices[0], vertices[1])
	G.AddEdge(vertices[0], vertices[2])

	G.AddEdge(vertices[1], vertices[2])
	G.AddEdge(vertices[1], vertices[3])

	G.AddEdge(vertices[2], vertices[3])

	G.AddEdge(vertices[3], vertices[8])
	G.AddEdge(vertices[3], vertices[4])

	G.AddEdge(vertices[4], vertices[5])
	G.AddEdge(vertices[4], vertices[7])

	G.AddEdge(vertices[5], vertices[6])
	G.AddEdge(vertices[6], vertices[7])

	G.AddEdge(vertices[8], vertices[9])

	return G
}

func NewGraph2() *conflictgraph.UndirectedGraph {
	G := conflictgraph.NewUndirectedGraph()
	vertices := make([]*conflictgraph.Vertex, 0)
	for i := 0; i < 6; i++ {
		vertices = append(vertices, &conflictgraph.Vertex{
			TxId:      i,
			IsDeleted: false,
			Degree:    0,
		})
		G.AddVertex(vertices[i])
	}
	G.AddEdge(vertices[0], vertices[1])

	G.AddEdge(vertices[1], vertices[2])
	G.AddEdge(vertices[1], vertices[3])

	G.AddEdge(vertices[2], vertices[4])
	G.AddEdge(vertices[2], vertices[5])

	G.AddEdge(vertices[3], vertices[4])
	G.AddEdge(vertices[3], vertices[5])

	G.AddEdge(vertices[4], vertices[5])

	return G
}

func TestSolveMIS(t *testing.T) {
	graph := NewGraph2()
	solution := NewSolution(graph)
	solution.Solve()
	vertices := solution.IndependentSet.ToSlice()
	for _, v := range vertices {
		t.Logf("%v", v.(*conflictgraph.Vertex).TxId)
	}
}
