package conflictgraph

import "github.com/ethereum/go-ethereum/common"

// Vertex 表示图中的顶点
type Vertex struct {
	TxId      int         // 顶点的 TxId
	TxHash    common.Hash // 顶点的 TxHash
	IsDeleted bool
	Degree    int
}

// UndirectedGraph 表示无向图
type UndirectedGraph struct {
	Vertices     []*Vertex             // 顶点集合
	AdjacencyMap map[*Vertex][]*Vertex // 邻接矩阵
}

// NewUndirectedGraph 创建一个新的无向图
func NewUndirectedGraph() *UndirectedGraph {
	return &UndirectedGraph{
		Vertices:     make([]*Vertex, 0),
		AdjacencyMap: make(map[*Vertex][]*Vertex),
	}
}

// AddVertex 向图中添加一个顶点
func (g *UndirectedGraph) AddVertex(v *Vertex) {
	g.Vertices = append(g.Vertices, v)
	g.AdjacencyMap[v] = make([]*Vertex, 0)
}

// AddEdge 向图中添加一条边
func (g *UndirectedGraph) AddEdge(source, destination *Vertex) {
	g.AdjacencyMap[source] = append(g.AdjacencyMap[source], destination)
	g.AdjacencyMap[destination] = append(g.AdjacencyMap[destination], source)
	source.Degree++
	destination.Degree++
}

func (g *UndirectedGraph) HasEdge(v1, v2 *Vertex) bool {
	if v1.IsDeleted || v2.IsDeleted {
		return false
	}
	for _, v := range g.AdjacencyMap[v1] {
		if v == v2 {
			return true
		}
	}
	return false
}

func (g *UndirectedGraph) RemoveVertex(v *Vertex) {
	v.IsDeleted = true
	for _, neighbor := range g.AdjacencyMap[v] {
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

	for _, neighbor := range g.AdjacencyMap[v] {
		if !visited[neighbor] && !neighbor.IsDeleted {
			g.dfs(neighbor, visited, component)
		}
	}
}
