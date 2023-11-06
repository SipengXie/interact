package mis

import (
	conflictgraph "interact/conflictGraph"

	set "github.com/deckarep/golang-set"
)

// 一个比较尴尬的事情是这个算法好像不一定是确定性的

type VertexStack []*conflictgraph.Vertex

func (s *VertexStack) Push(v *conflictgraph.Vertex) {
	*s = append(*s, v)
}

func (s *VertexStack) Pop() *conflictgraph.Vertex {
	old := *s
	n := len(old)
	if n == 0 {
		return nil
	}
	v := old[n-1]
	*s = old[0 : n-1]
	return v
}

type LinearTime struct {
	Graph *conflictgraph.UndirectedGraph

	VerticesOne, VerticesTwo, VerticesGreaterThanThree, IndependentSet set.Set // 存指针

	Stack VertexStack
}

func NewSolution(graph *conflictgraph.UndirectedGraph) *LinearTime {
	VerticesOne := set.NewSet()
	VerticesTwo := set.NewSet()
	VerticesGreaterThanThree := set.NewSet()
	IndependentSet := set.NewSet()
	Stack := make([]*conflictgraph.Vertex, 0)

	for _, v := range graph.Vertices {
		switch v.Degree {
		case 0:
			IndependentSet.Add(v)
			v.IsDeleted = true
		case 1:
			VerticesOne.Add(v)
		case 2:
			VerticesTwo.Add(v)
		default:
			VerticesGreaterThanThree.Add(v)
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
	for u := s.Stack.Pop(); u != nil; u = s.Stack.Pop() {
		canAdd := true
		for _, neighbor := range s.Graph.AdjacencyMap[u] {
			if !neighbor.IsDeleted && !s.IndependentSet.Contains(neighbor) {
				continue
			} else {
				canAdd = false
				break
			}
		}
		if canAdd {
			s.IndependentSet.Add(u)
		}
	}
}

func (s *LinearTime) deleteVertex(v *conflictgraph.Vertex) {
	v.IsDeleted = true
	if v.Degree == 0 {
		return
	}
	for _, neighbor := range s.Graph.AdjacencyMap[v] {
		if !neighbor.IsDeleted {
			neighbor.Degree--
			switch neighbor.Degree {
			case 0:
				s.IndependentSet.Add(neighbor)
				s.VerticesOne.Remove(neighbor)
			case 1:
				s.VerticesOne.Add(neighbor)
				s.VerticesTwo.Remove(neighbor)
			case 2:
				s.VerticesTwo.Add(neighbor)
				s.VerticesGreaterThanThree.Remove(neighbor)
			}
		}
	}
}

func (s *LinearTime) degreeOneReduction() {
	v := s.VerticesOne.Pop()
	s.deleteVertex(v.(*conflictgraph.Vertex))
}

func (s *LinearTime) inexactReduction() {
	var maxDegree = 0
	var maxDegreeVertex *conflictgraph.Vertex = nil

	for _, v := range s.VerticesGreaterThanThree.ToSlice() {
		vertex := v.(*conflictgraph.Vertex)
		if vertex.Degree > maxDegree {
			maxDegree = vertex.Degree
			maxDegreeVertex = vertex
		}
	}

	if maxDegreeVertex != nil {
		s.deleteVertex(maxDegreeVertex)
	}
}

func (s *LinearTime) degreeTwoPathReduction() {
	u := s.VerticesTwo.Pop()
	path, isCycle := s.findLongestDegreeTwoPath(u.(*conflictgraph.Vertex))
	path = s.pathReOrg(path)

	if isCycle {
		s.deleteVertex(u.(*conflictgraph.Vertex))
	} else {

		var v, w *conflictgraph.Vertex = nil, nil
		for _, neighbor := range s.Graph.AdjacencyMap[path[0]] {
			if !neighbor.IsDeleted && neighbor.Degree != 2 {
				v = neighbor
				break
			}
		}

		for _, neighbor := range s.Graph.AdjacencyMap[path[len(path)-1]] {
			if !neighbor.IsDeleted && neighbor.Degree != 2 {
				w = neighbor
				break
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
					s.Graph.RemoveVertex(path[i])
					s.VerticesTwo.Remove(path[i])
				}
				s.Graph.AddEdge(path[0], w)
				for i := len(path) - 1; i > 0; i-- {
					s.Stack.Push(path[i])
				}
			}
		} else {
			for _, v := range path {
				s.Graph.RemoveVertex(v)
				s.VerticesTwo.Remove(v)
			}
			if !s.Graph.HasEdge(v, w) {
				s.Graph.AddEdge(v, w)
			}
			for i := len(path) - 1; i >= 0; i-- {
				s.Stack.Push(path[i])
			}
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
			for _, neighbor := range s.Graph.AdjacencyMap[v] {
				if neighbor.Degree != 2 && !neighbor.IsDeleted {
					st = neighbor
					break
				}
			}
		}
	}

	path := make([]*conflictgraph.Vertex, 0)
	s.dfsToReOrgPath(st, visited, inPath, &path)
	return path
}

func (s *LinearTime) findLongestDegreeTwoPath(v *conflictgraph.Vertex) ([]*conflictgraph.Vertex, bool) {
	visited := make(map[*conflictgraph.Vertex]bool)
	longestPath := make([]*conflictgraph.Vertex, 0)
	isCycle := true

	s.dfsToFindDegreeTwoPath(v, visited, &longestPath)
	for _, vertex := range longestPath {
		for _, neighbor := range s.Graph.AdjacencyMap[vertex] {
			if !visited[neighbor] && !neighbor.IsDeleted {
				isCycle = false
				break
			}
		}
	}

	return longestPath, isCycle
}

func (s *LinearTime) dfsToFindDegreeTwoPath(vertex *conflictgraph.Vertex, visited map[*conflictgraph.Vertex]bool, path *[]*conflictgraph.Vertex) {
	visited[vertex] = true
	*path = append(*path, vertex)

	for _, neighbor := range s.Graph.AdjacencyMap[vertex] {
		if !visited[neighbor] && neighbor.Degree == 2 && !neighbor.IsDeleted {
			s.dfsToFindDegreeTwoPath(neighbor, visited, path)
		}
	}
}

func (s *LinearTime) dfsToReOrgPath(vertex *conflictgraph.Vertex, visited map[*conflictgraph.Vertex]bool, inPath map[*conflictgraph.Vertex]bool, path *[]*conflictgraph.Vertex) {
	visited[vertex] = true
	*path = append(*path, vertex)

	for _, neighbor := range s.Graph.AdjacencyMap[vertex] {
		if !visited[neighbor] && !neighbor.IsDeleted && inPath[neighbor] {
			s.dfsToReOrgPath(neighbor, visited, inPath, path)
		}
	}
}
