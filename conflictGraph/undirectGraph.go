package conflictgraph

import (
	"github.com/ethereum/go-ethereum/common"
)

// Vertex 表示图中的顶点
type Vertex struct {
	TxId      uint        `json:"txId"`   // 顶点的 TxId
	TxHash    common.Hash `json:"txHash"` // 顶点的 TxHash
	IsDeleted bool        `json:"isDeleted"`
	Degree    uint        `json:"degree"` // 顶点的度
}

// UndirectedGraph 表示无向图
type UndirectedGraph struct {
	Vertices     map[uint]*Vertex `json:"vertices"`     // 顶点集合
	AdjacencyMap map[uint][]uint  `json:"adjacencyMap"` // 邻接边表
}

// NewUndirectedGraph 创建一个新的无向图
func NewUndirectedGraph() *UndirectedGraph {
	return &UndirectedGraph{
		Vertices:     make(map[uint]*Vertex),
		AdjacencyMap: make(map[uint][]uint),
	}
}

func (g *UndirectedGraph) CopyGraphWithDeletion() *UndirectedGraph {
	NewG := NewUndirectedGraph()
	for id, v := range g.Vertices {
		if !v.IsDeleted {
			NewG.AddVertex(v.TxHash, id)
		}
	}
	for id := range NewG.Vertices {
		for _, neighborId := range g.AdjacencyMap[id] {
			neighbor := g.Vertices[neighborId]
			if !neighbor.IsDeleted {
				NewG.AddEdge(id, neighbor.TxId)
			}
		}
	}
	return NewG
}

// AddVertex 向图中添加一个顶点
func (g *UndirectedGraph) AddVertex(tx common.Hash, id uint) {
	_, exist := g.Vertices[id]
	if exist {
		return
	}
	v := &Vertex{
		TxId:      id,
		TxHash:    tx,
		IsDeleted: false,
		Degree:    0,
	}
	g.Vertices[id] = v
	g.AdjacencyMap[id] = make([]uint, 0)
}

// AddEdge 向图中添加一条边
func (g *UndirectedGraph) AddEdge(source, destination uint) {
	if g.HasEdge(source, destination) {
		return
	}
	g.AdjacencyMap[source] = append(g.AdjacencyMap[source], destination)
	g.AdjacencyMap[destination] = append(g.AdjacencyMap[destination], source)
	g.Vertices[source].Degree++
	g.Vertices[destination].Degree++
}

func (g *UndirectedGraph) HasEdge(tx1, tx2 uint) bool {
	v1 := g.Vertices[tx1]
	v2 := g.Vertices[tx2]
	if v1.IsDeleted || v2.IsDeleted {
		return false
	}
	for _, tx := range g.AdjacencyMap[tx1] {
		if tx == tx2 {
			return true
		}
	}
	return false
}

func (g *UndirectedGraph) RemoveVertex(tx uint) {
	v := g.Vertices[tx]
	v.IsDeleted = true
	for _, neighborTx := range g.AdjacencyMap[tx] {
		neighbor := g.Vertices[neighborTx]
		if !neighbor.IsDeleted {
			neighbor.Degree--
		}
	}
}

// GetConnectedComponents 获取图中的连通分量（使用深度优先搜索）
func (g *UndirectedGraph) GetConnectedComponents() [][]*Vertex {
	visited := make(map[*Vertex]bool)
	components := [][]*Vertex{}

	for _, v := range g.Vertices {
		if !visited[v] && !v.IsDeleted {
			component := []*Vertex{}
			g.dfs(v, visited, &component)
			components = append(components, component)
		}
	}

	return components
}

// dfs 深度优先搜索函数
func (g *UndirectedGraph) dfs(v *Vertex, visited map[*Vertex]bool, component *[]*Vertex) {
	visited[v] = true
	*component = append(*component, v)

	for _, neighborId := range g.AdjacencyMap[v.TxId] {
		neighbor := g.Vertices[neighborId]
		if !visited[neighbor] && !neighbor.IsDeleted {
			g.dfs(neighbor, visited, component)
		}
	}
}
