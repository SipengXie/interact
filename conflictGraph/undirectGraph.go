package conflictgraph

import "github.com/ethereum/go-ethereum/common"

// Vertex 表示图中的顶点
type Vertex struct {
	TxId   int         // 顶点的 TxId
	TxHash common.Hash // 顶点的 TxHash
}

// UndirectedGraph 表示无向图
type UndirectedGraph struct {
	vertices     []*Vertex             // 顶点集合
	adjacencyMap map[*Vertex][]*Vertex // 邻接矩阵
}

// NewUndirectedGraph 创建一个新的无向图
func NewUndirectedGraph() *UndirectedGraph {
	return &UndirectedGraph{
		vertices:     make([]*Vertex, 0),
		adjacencyMap: make(map[*Vertex][]*Vertex),
	}
}

// AddVertex 向图中添加一个顶点
func (g *UndirectedGraph) AddVertex(v *Vertex) {
	g.vertices = append(g.vertices, v)
	g.adjacencyMap[v] = make([]*Vertex, 0)
}

// AddEdge 向图中添加一条边
func (g *UndirectedGraph) AddEdge(source, destination *Vertex) {
	g.adjacencyMap[source] = append(g.adjacencyMap[source], destination)
	g.adjacencyMap[destination] = append(g.adjacencyMap[destination], source)
}

// GetConnectedComponents 获取图中的连通分量（使用深度优先搜索）
func (g *UndirectedGraph) GetConnectedComponents() [][]*Vertex {
	visited := make(map[*Vertex]bool)
	components := [][]*Vertex{}

	for _, v := range g.vertices {
		if !visited[v] {
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

	for _, neighbor := range g.adjacencyMap[v] {
		if !visited[neighbor] {
			g.dfs(neighbor, visited, component)
		}
	}
}
