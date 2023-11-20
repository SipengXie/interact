package conflictgraph

import "github.com/ethereum/go-ethereum/common"

// UndirectedGraph 表示无向图
type DirectedGraph struct {
	Vertices     map[uint]*Vertex           `json:"vertices"`     // 顶点集合
	AdjacencyMap map[uint]map[uint]struct{} `json:"adjacencyMap"` // 邻接边表
}

func NewDirectedGraph() *DirectedGraph {
	return &DirectedGraph{
		Vertices:     make(map[uint]*Vertex),
		AdjacencyMap: make(map[uint]map[uint]struct{}),
	}
}

func (g *DirectedGraph) AddVertex(tx common.Hash, id uint) {
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
	g.AdjacencyMap[id] = make(map[uint]struct{})
}

func (g *DirectedGraph) AddEdge(source, destination uint) {
	if g.HasEdge(source, destination) {
		return
	}
	g.AdjacencyMap[source][destination] = struct{}{}
	g.Vertices[destination].Degree++
}

func (g *DirectedGraph) HasEdge(source, destination uint) bool {
	_, ok := g.AdjacencyMap[source][destination]
	return ok
}

func (g *DirectedGraph) GetDegreeZero() [][]uint {
	ans := make([][]uint, 0)
	degreeZero := make([]uint, 0)
	for id, v := range g.Vertices {
		if v.Degree == 0 {
			degreeZero = append(degreeZero, id)
		}
	}
	ans = append(ans, degreeZero)
	for {
		newDegreeZero := make([]uint, 0)
		for _, vid := range degreeZero {
			for neighborid, _ := range g.AdjacencyMap[vid] {
				g.Vertices[neighborid].Degree--
				if g.Vertices[neighborid].Degree == 0 {
					newDegreeZero = append(newDegreeZero, neighborid)
				}
			}
		}
		degreeZero = newDegreeZero
		if len(degreeZero) == 0 {
			break
		} else {
			ans = append(ans, degreeZero)
		}
	}
	return ans
}
