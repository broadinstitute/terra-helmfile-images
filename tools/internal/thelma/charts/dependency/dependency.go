package dependency

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"sort"
	"strings"
)

// Graph dependency graph for local charts in a source directory
type Graph struct {
	nodes map[string]graphNode
	topoOrder map[string]int
}

// graphNode node in a dependency graph
type graphNode struct {
	chartName    string
	dependencies []graphNode
	dependents   []graphNode
}

// NewGraph constructor for a dependency graph
func NewGraph(dir *source.Dir) (*Graph, error) {
	chartNames := dir.ChartNames()

	// Make a node for each chart
	nodes := make(map[string]graphNode, len(chartNames))
	for _, chartName := range dir.ChartNames() {
		nodes[chartName] = graphNode{chartName: chartName}
	}

	// Populate dependency relationships for nodes in the graph
	for chartName, node := range nodes {
		localDeps, err := dir.LocalDependencies(chartName)
		if err != nil {
			return nil, err
		}
		for _, depName := range localDeps {
			depNode := nodes[depName]

			// Add dependency relationships
			node.dependencies = append(node.dependencies, depNode)
			depNode.dependents = append(depNode.dependents, node)

			// Update graph
			nodes[node.chartName] = node
			nodes[depNode.chartName] = depNode
		}
	}

	if err := checkForCycles(nodes); err != nil {
		return nil, err
	}

	topoOrder := computeTopoOrdering(nodes)

	return &Graph{ nodes: nodes, topoOrder: topoOrder }, nil
}

// TopoSort will sort the given charts in topological order.
// Eg. suppose we have
// A <- B <- C       (C depends on B, which depends on A)
// D <- E
// G
// H <- I <- J
//
// TopoSort("C", "H", "E", "I", "B", "D") will return
// []{"D", "B", "H", "I", "C", "E"}
// (or another valid topological ordering)
//
// This sort is not stable.
func (graph *Graph) TopoSort(chartNames []string) {
	sort.Slice(chartNames, func(i, j int) bool {
		return graph.topoOrder[chartNames[i]] < graph.topoOrder[chartNames[j]]
	})
}

// Given a set of chart names, return the charts along with names of all transitive local dependents
// Eg. suppose we have
// A <- B <- C       (C depends on B, which depends on A)
// D <- E
// G
// H <- I <- J
// WithDependents([]string{"A", "G", "I"}) will return []string{"A", "G", "B", "C", "I", "J"}
func (graph *Graph) WithDependents(chartNames []string) []string {
	queue := make([]string, 0, len(chartNames))
	visited := make(map[string]bool)
	result := make([]string, 0, len(chartNames))

	// Add all charts to the queue and mark visited
	for _, chart := range chartNames {
		queue = append(queue, chart)
		visited[chart] = true
	}

	// BFS: until the queue is empty, pop off the first element,
	// add it to the result, and add unvisited dependents to the queue
	for len(queue) > 0 {
		chart := queue[0]
		queue = queue[1:]

		result = append(result, chart)
		node := graph.nodes[chart]

		for _, dep := range node.dependents {
			if !visited[dep.chartName] {
				visited[dep.chartName] = true
				queue = append(queue, dep.chartName)
			}
		}
	}

	return result
}

// compute a topological ordering of lal nodes in the graph
func computeTopoOrdering(nodes map[string]graphNode) map[string]int {
	dependencyCounts := make(map[string]int, len(nodes))
	queue := make([]graphNode, 0, len(nodes))

	for chartName, node := range nodes {
		dependencyCounts[chartName] = len(node.dependencies)
		if len(node.dependencies) == 0 {
			queue = append(queue, node)
		}
	}

	topoOrder := make(map[string]int)
	topoCounter := 0
	for len(queue) > 0 {
		currentNode := queue[0]
		queue = queue[1:]

		topoOrder[currentNode.chartName] = topoCounter
		topoCounter++

		for _, dependent := range currentNode.dependents {
			// Decrement dependency count
			depCount := dependencyCounts[dependent.chartName]
			depCount--
			dependencyCounts[dependent.chartName] = depCount
			if depCount == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return topoOrder
}

func checkForCycles(nodes map[string]graphNode) error {
	checked := make(map[string]bool, len(nodes))
	pathMap := make(map[string]string)
	for _, node := range nodes {
		if err := searchForCycles(node, pathMap, checked); err != nil {
			return err
		}
	}
	return nil
}

func searchForCycles(node graphNode, path map[string]string, checked map[string]bool) error {
	if checked[node.chartName] {
		return nil
	}
	for _, dep := range node.dependencies {
		if _, exists := path[dep.chartName]; exists {
			return fmt.Errorf("cycle detected: %s", pathToString(path, node.chartName, dep.chartName))
		}

		path[dep.chartName] = node.chartName
		if err := searchForCycles(dep, path, checked); err != nil {
			return err
		}
		delete(path, dep.chartName)
	}
	checked[node.chartName] = true
	return nil
}

func pathToString(path map[string]string, lastElement string, repeatElement string) string {
	pathList := []string{repeatElement}
	currentElement := lastElement
	for {
		pathList = append(pathList, currentElement)
		parent, exists := path[currentElement]
		if !exists {
			break
		}
		currentElement = parent
	}

	reverse(pathList)

	return strings.Join(pathList, " -> ")
}

func reverse(s []string) {
	for i, j := 0, len(s) - 1; i < len(s) / 2; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}