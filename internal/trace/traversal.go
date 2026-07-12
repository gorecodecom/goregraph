package trace

import (
	"fmt"
	"sort"

	"github.com/gorecodecom/goregraph/internal/scan"
)

type Options struct {
	MaxNodes       int
	MaxCycleVisits int
}
type Result struct {
	Focus              string     `json:"focus"`
	Upstream           []string   `json:"upstream"`
	Downstream         []string   `json:"downstream"`
	ShortestEntryPath  []string   `json:"shortest_entry_path,omitempty"`
	PathsToPersistence [][]string `json:"paths_to_persistence,omitempty"`
	PathsToMessages    [][]string `json:"paths_to_messages,omitempty"`
	PathsToExternal    [][]string `json:"paths_to_external,omitempty"`
	PathsToTests       [][]string `json:"paths_to_tests,omitempty"`
	Branches           []string   `json:"branches,omitempty"`
	Cycles             [][]string `json:"cycles,omitempty"`
	Truncated          bool       `json:"truncated"`
}

func Traverse(graph scan.DirectedTraceRecord, focus string, options Options) (Result, error) {
	if options.MaxNodes == 0 {
		options.MaxNodes = 100
	}
	if options.MaxCycleVisits == 0 {
		options.MaxCycleVisits = 3
	}
	nodes := map[string]scan.DirectedTraceNodeRecord{}
	forward := map[string][]string{}
	reverse := map[string][]string{}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	if _, ok := nodes[focus]; !ok {
		return Result{}, fmt.Errorf("trace node %q not found", focus)
	}
	for _, edge := range graph.Edges {
		forward[edge.From] = append(forward[edge.From], edge.To)
		reverse[edge.To] = append(reverse[edge.To], edge.From)
	}
	sortAdj(forward)
	sortAdj(reverse)
	up, upTruncated := boundedReachable(focus, reverse, options.MaxNodes)
	down, downTruncated := boundedReachable(focus, forward, options.MaxNodes)
	result := Result{Focus: focus, Upstream: up, Downstream: down, Truncated: upTruncated || downTruncated}
	result.ShortestEntryPath = shortestToAny(focus, reverse, setOf(graph.EntryNodes))
	result.PathsToPersistence = pathsToRoles(focus, forward, nodes, map[scan.TraceNodeRole]bool{scan.TraceRoleRepository: true, scan.TraceRoleDatabase: true}, options.MaxNodes)
	result.PathsToMessages = pathsToRoles(focus, forward, nodes, map[scan.TraceNodeRole]bool{scan.TraceRoleMessageProducer: true, scan.TraceRoleChannel: true, scan.TraceRoleMessageConsumer: true}, options.MaxNodes)
	result.PathsToExternal = pathsToRoles(focus, forward, nodes, map[scan.TraceNodeRole]bool{scan.TraceRoleExternal: true}, options.MaxNodes)
	result.PathsToTests = pathsToRoles(focus, forward, nodes, map[scan.TraceNodeRole]bool{scan.TraceRoleTest: true}, options.MaxNodes)
	for node, targets := range forward {
		if len(targets) > 1 {
			result.Branches = append(result.Branches, node)
		}
	}
	sort.Strings(result.Branches)
	result.Cycles = findCycles(forward, options.MaxCycleVisits)
	return result, nil
}

func boundedReachable(start string, adj map[string][]string, limit int) ([]string, bool) {
	seen := map[string]bool{start: true}
	queue := []string{start}
	result := []string{}
	truncated := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adj[current] {
			if seen[next] {
				continue
			}
			if len(result) >= limit {
				truncated = true
				continue
			}
			seen[next] = true
			result = append(result, next)
			queue = append(queue, next)
		}
	}
	sort.Strings(result)
	return result, truncated
}
func shortestToAny(start string, adj map[string][]string, targets map[string]bool) []string {
	if targets[start] {
		return []string{start}
	}
	queue := [][]string{{start}}
	seen := map[string]bool{start: true}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		for _, next := range adj[path[len(path)-1]] {
			if seen[next] {
				continue
			}
			candidate := append(append([]string{}, path...), next)
			if targets[next] {
				return candidate
			}
			seen[next] = true
			queue = append(queue, candidate)
		}
	}
	return nil
}
func pathsToRoles(start string, adj map[string][]string, nodes map[string]scan.DirectedTraceNodeRecord, roles map[scan.TraceNodeRole]bool, limit int) [][]string {
	results := [][]string{}
	for id, node := range nodes {
		if !roles[node.Role] {
			continue
		}
		if path := shortestToAny(start, adj, map[string]bool{id: true}); len(path) > 0 && len(path) <= limit+1 {
			results = append(results, path)
		}
	}
	sort.Slice(results, func(i, j int) bool { return fmt.Sprint(results[i]) < fmt.Sprint(results[j]) })
	return results
}
func findCycles(adj map[string][]string, maxVisits int) [][]string {
	cycles := [][]string{}
	seenKeys := map[string]bool{}
	var walk func(string, []string, map[string]int)
	walk = func(node string, path []string, visits map[string]int) {
		if visits[node] >= maxVisits {
			return
		}
		for _, next := range adj[node] {
			for i, id := range path {
				if id == next {
					cycle := append(append([]string{}, path[i:]...), next)
					key := fmt.Sprint(cycle)
					if !seenKeys[key] {
						seenKeys[key] = true
						cycles = append(cycles, cycle)
					}
					continue
				}
			}
			copyVisits := map[string]int{}
			for k, v := range visits {
				copyVisits[k] = v
			}
			copyVisits[next]++
			walk(next, append(append([]string{}, path...), next), copyVisits)
		}
	}
	keys := []string{}
	for key := range adj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		walk(key, []string{key}, map[string]int{key: 1})
	}
	return cycles
}
func sortAdj(adj map[string][]string) {
	for key := range adj {
		sort.Strings(adj[key])
	}
}
func setOf(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
