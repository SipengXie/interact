package mis

import (
	"fmt"
	conflictgraph "interact/conflictGraph"

	set "github.com/deckarep/golang-set"
)

// 一个比较尴尬的事情是这个算法好像不一定是确定性的
// 不确定性来自于我们Set存储的都是指针，而指针大小是不确定的
// 且Set的底层是Map，可能造成不一致的Pop；
// 实际上，我们可以通过修改数据结构以及存储的数据来保证一致性
const MAX_UINT = uint(2147483647)

type VertexStack []uint

func (s *VertexStack) Push(v uint) {
	*s = append(*s, v)
}

func (s *VertexStack) Pop() uint {
	old := *s
	n := len(old)
	if n == 0 {
		return MAX_UINT
	}
	v := old[n-1]
	*s = old[0 : n-1]
	return v
}

type LinearTime struct {
	Graph *conflictgraph.UndirectedGraph

	VerticesOne, VerticesTwo, VerticesGreaterThanThree, IndependentSet set.Set // 存txID

	Stack VertexStack
}

func NewSolution(graph *conflictgraph.UndirectedGraph) *LinearTime {
	VerticesOne := set.NewSet()
	VerticesTwo := set.NewSet()
	VerticesGreaterThanThree := set.NewSet()
	IndependentSet := set.NewSet()
	Stack := make([]uint, 0)

	for _, v := range graph.Vertices {
		switch v.Degree {
		case 0:
			IndependentSet.Add(v.TxId)
			v.IsDeleted = true
		case 1:
			VerticesOne.Add(v.TxId)
		case 2:
			VerticesTwo.Add(v.TxId)
		default:
			VerticesGreaterThanThree.Add(v.TxId)
		}
	}

	return &LinearTime{
		Graph:                    graph,
		VerticesOne:              VerticesOne,
		VerticesTwo:              VerticesTwo,
		VerticesGreaterThanThree: VerticesGreaterThanThree,
		IndependentSet:           IndependentSet,
		Stack:                    Stack,
	}
}

func (s *LinearTime) Solve() {
	for s.VerticesOne.Cardinality() > 0 || s.VerticesTwo.Cardinality() > 0 || s.VerticesGreaterThanThree.Cardinality() > 0 {
		if s.VerticesOne.Cardinality() > 0 {
			s.degreeOneReduction()
		} else if s.VerticesTwo.Cardinality() > 0 {
			s.degreeTwoPathReduction()
		} else {
			s.inexactReduction()
		}
	}
	for id := s.Stack.Pop(); id != MAX_UINT; id = s.Stack.Pop() {
		canAdd := true
		for _, neighborId := range s.Graph.AdjacencyMap[id] {
			neighbor := s.Graph.Vertices[neighborId]
			if !neighbor.IsDeleted && !s.IndependentSet.Contains(neighborId) {
				continue
			} else {
				canAdd = false
				break
			}
		}
		if canAdd {
			s.IndependentSet.Add(id)
		}
	}
}

func (s *LinearTime) deleteVertex(id uint) {
	v := s.Graph.Vertices[id]
	v.IsDeleted = true
	switch v.Degree {
	case 1:
		s.VerticesOne.Remove(id)
	case 2:
		s.VerticesTwo.Remove(id)
	default:
		s.VerticesGreaterThanThree.Remove(id)
	}
	if v.Degree == 0 {
		return
	}
	for _, neighborId := range s.Graph.AdjacencyMap[id] {
		neighbor := s.Graph.Vertices[neighborId]
		if !neighbor.IsDeleted {
			neighbor.Degree--
			switch neighbor.Degree {
			case 0:
				s.IndependentSet.Add(neighborId)
				s.VerticesOne.Remove(neighborId)
			case 1:
				s.VerticesOne.Add(neighborId)
				s.VerticesTwo.Remove(neighborId)
			case 2:
				s.VerticesTwo.Add(neighborId)
				s.VerticesGreaterThanThree.Remove(neighborId)
			}
		}
	}
}

func (s *LinearTime) degreeOneReduction() {
	txId := s.VerticesOne.Pop().(uint)
	for _, neighborId := range s.Graph.AdjacencyMap[txId] {
		neighbor := s.Graph.Vertices[neighborId]
		if !neighbor.IsDeleted {
			s.deleteVertex(neighborId)
		}
	}
}

func (s *LinearTime) inexactReduction() {
	var maxDegree = uint(0)
	var maxDegreeId = MAX_UINT

	for _, txId := range s.VerticesGreaterThanThree.ToSlice() {
		vertex := s.Graph.Vertices[txId.(uint)]
		if vertex.Degree > maxDegree {
			maxDegree = vertex.Degree
			maxDegreeId = txId.(uint)
		}
	}

	if maxDegreeId != MAX_UINT {
		s.deleteVertex(maxDegreeId)
	}
}

func (s *LinearTime) degreeTwoPathReduction() {
	uId := s.VerticesTwo.Pop().(uint)
	path, isCycle := s.findLongestDegreeTwoPath(uId)

	if isCycle {
		s.deleteVertex(uId)
	} else {
		path = s.pathReOrg(path)
		var v, w uint = MAX_UINT, MAX_UINT
		if len(path) == 1 {
			for _, neighborId := range s.Graph.AdjacencyMap[path[0].TxId] {
				neighbor := s.Graph.Vertices[neighborId]
				if !neighbor.IsDeleted && neighbor.Degree != 2 {
					if v == MAX_UINT {
						v = neighbor.TxId
					} else if w == MAX_UINT {
						w = neighbor.TxId
					} else {
						break
					}
				}
			}
		} else {
			for _, neighborId := range s.Graph.AdjacencyMap[path[0].TxId] {
				neighbor := s.Graph.Vertices[neighborId]
				if !neighbor.IsDeleted && neighbor.Degree != 2 {
					v = neighbor.TxId
					break
				}
			}

			for _, neighborId := range s.Graph.AdjacencyMap[path[len(path)-1].TxId] {
				neighbor := s.Graph.Vertices[neighborId]
				if !neighbor.IsDeleted && neighbor.Degree != 2 {
					w = neighbor.TxId
					break
				}
			}
		}

		if v == w {
			s.deleteVertex(v)
		} else if len(path)%2 == 1 {
			if s.Graph.HasEdge(v, w) {
				s.deleteVertex(v)
				s.deleteVertex(w)
			} else {
				for i := 1; i < len(path); i++ {
					s.Graph.RemoveVertex(path[i].TxId)
					s.VerticesTwo.Remove(path[i].TxId)
				}
				s.Graph.AddEdge(path[0].TxId, w)
				for i := len(path) - 1; i > 0; i-- {
					s.Stack.Push(path[i].TxId)
				}
			}
		} else {
			for _, vertex := range path {
				s.Graph.RemoveVertex(vertex.TxId)
				s.VerticesTwo.Remove(vertex.TxId)
			}
			fmt.Println("v:", v)
			fmt.Println("w:", w)
			if v == 73 && w == MAX_UINT {
				fmt.Println("Bingo")
			}
			if !s.Graph.HasEdge(v, w) {
				s.Graph.AddEdge(v, w)
			}
			for i := len(path) - 1; i >= 0; i-- {
				s.Stack.Push(path[i].TxId)
			}
		}
	}
}

func (s *LinearTime) findLongestDegreeTwoPath(vId uint) ([]*conflictgraph.Vertex, bool) {
	visited := make(map[*conflictgraph.Vertex]bool)
	longestPath := make([]*conflictgraph.Vertex, 0)
	isCycle := true

	s.dfsToFindDegreeTwoPath(vId, visited, &longestPath)
	for _, vertex := range longestPath {
		vId := vertex.TxId
		for _, neighborId := range s.Graph.AdjacencyMap[vId] {
			neighbor := s.Graph.Vertices[neighborId]
			if !visited[neighbor] && !neighbor.IsDeleted {
				isCycle = false
				break
			}
		}
		if !isCycle {
			break
		}
	}

	return longestPath, isCycle
}

func (s *LinearTime) dfsToFindDegreeTwoPath(vId uint, visited map[*conflictgraph.Vertex]bool, path *[]*conflictgraph.Vertex) {
	vertex := s.Graph.Vertices[vId]
	visited[vertex] = true
	*path = append(*path, vertex)

	for _, neighborId := range s.Graph.AdjacencyMap[vId] {
		neighbor := s.Graph.Vertices[neighborId]
		if !visited[neighbor] && neighbor.Degree == 2 && !neighbor.IsDeleted {
			s.dfsToFindDegreeTwoPath(neighborId, visited, path)
		}
	}
}

func (s *LinearTime) pathReOrg(initPath []*conflictgraph.Vertex) []*conflictgraph.Vertex {
	inPath := make(map[*conflictgraph.Vertex]bool)
	visited := make(map[*conflictgraph.Vertex]bool)
	var st *conflictgraph.Vertex = nil
	for _, v := range initPath {
		inPath[v] = true
		if st == nil {
			vId := v.TxId
			for _, neighborId := range s.Graph.AdjacencyMap[vId] {
				neighbor := s.Graph.Vertices[neighborId]
				if neighbor.Degree != 2 && !neighbor.IsDeleted {
					st = v
					break
				}
			}
		}
	}

	path := make([]*conflictgraph.Vertex, 0)
	s.dfsToReOrgPath(st, visited, inPath, &path)
	return path
}

func (s *LinearTime) dfsToReOrgPath(vertex *conflictgraph.Vertex, visited map[*conflictgraph.Vertex]bool, inPath map[*conflictgraph.Vertex]bool, path *[]*conflictgraph.Vertex) {
	visited[vertex] = true
	*path = append(*path, vertex)
	vId := vertex.TxId
	for _, neighborId := range s.Graph.AdjacencyMap[vId] {
		neighbor := s.Graph.Vertices[neighborId]
		if !visited[neighbor] && !neighbor.IsDeleted && inPath[neighbor] {
			s.dfsToReOrgPath(neighbor, visited, inPath, path)
		}
	}
}
