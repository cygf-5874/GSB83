// Package minreorder keeps a set of integer nodes together with
// "before -> after" dependency relations and produces an order that
// satisfies every relation.
//
// The order reported by Order is the one that keeps as much of the previous
// output as possible (the stable, minimal-churn order): among all orders that
// satisfy the dependencies it keeps a longest common subsequence with the
// previous output, breaking ties by the lexicographically smallest sequence.
package minreorder

import (
	"errors"
	"sort"
)

var (
	// ErrCycle is returned when the requested dependencies contain a cycle.
	ErrCycle = errors.New("minreorder: dependency cycle")
	// ErrBadDep is returned when a dependency refers to an unknown or removed node.
	ErrBadDep = errors.New("minreorder: dependency refers to an unknown or removed node")
)

// Orderer holds the nodes and dependency relations.
//
// This implementation is correct but rebuilds the whole arrangement on every
// call to Order.
type Orderer struct {
	nodes map[int]bool
	succ  map[int][]int // succ[a] = nodes that must come after a
	pred  map[int][]int // pred[b] = nodes that must come before b
	prev  []int         // previous output order (baseline for minimal churn)
	stats Stats
}

// NewOrderer builds an orderer over ids. Each element of deps is {before, after}
// meaning "before" must appear before "after". It returns ErrBadDep for an
// unknown or self dependency and ErrCycle when the dependencies contain a cycle.
func NewOrderer(ids []int, deps [][2]int) (*Orderer, error) {
	o := &Orderer{
		nodes: make(map[int]bool, len(ids)),
		succ:  make(map[int][]int),
		pred:  make(map[int][]int),
	}
	for _, id := range ids {
		if o.nodes[id] {
			return nil, ErrBadDep
		}
		o.nodes[id] = true
	}

	type edge struct{ a, b int }
	seen := make(map[edge]bool, len(deps))
	for _, d := range deps {
		if !o.nodes[d[0]] || !o.nodes[d[1]] || d[0] == d[1] {
			return nil, ErrBadDep
		}
		e := edge{d[0], d[1]}
		if seen[e] {
			continue
		}
		seen[e] = true
		o.succ[d[0]] = append(o.succ[d[0]], d[1])
		o.pred[d[1]] = append(o.pred[d[1]], d[0])
	}
	for k := range o.succ {
		sort.Ints(o.succ[k])
	}
	for k := range o.pred {
		sort.Ints(o.pred[k])
	}
	if o.hasCycle() {
		return nil, ErrCycle
	}

	o.prev = append([]int(nil), ids...)
	o.stats.Allocs++
	return o, nil
}

// Insert adds a new node id. Every element of after must already exist and the
// new node is placed after all of them. It returns ErrBadDep when a referenced
// node is unknown or was removed, and ErrCycle when the insertion would create
// a cycle. On error the orderer is left unchanged.
func (o *Orderer) Insert(id int, after []int) error {
	if o.nodes[id] {
		return ErrBadDep
	}
	uniq := make(map[int]bool, len(after))
	for _, a := range after {
		if a == id {
			return ErrCycle
		}
		if !o.nodes[a] {
			return ErrBadDep
		}
		uniq[a] = true
	}

	// id is brand new and has no outgoing edges yet, so adding after->id
	// cannot close a cycle. Apply the change.
	o.nodes[id] = true
	o.stats.Allocs++
	for a := range uniq {
		o.succ[a] = append(o.succ[a], id)
		o.pred[id] = append(o.pred[id], a)
	}
	for a := range uniq {
		sort.Ints(o.succ[a])
	}
	sort.Ints(o.pred[id])
	o.prev = append(o.prev, id)
	return nil
}

// Delete removes an existing node and all of its dependency relations. It
// returns ErrBadDep when the node is unknown.
func (o *Orderer) Delete(id int) error {
	if !o.nodes[id] {
		return ErrBadDep
	}
	o.nodes[id] = false
	o.stats.Allocs++

	for _, v := range o.succ[id] {
		o.pred[v] = removeInt(o.pred[v], id)
	}
	for _, p := range o.pred[id] {
		o.succ[p] = removeInt(o.succ[p], id)
	}
	delete(o.succ, id)
	delete(o.pred, id)
	for i, x := range o.prev {
		if x == id {
			o.prev = append(o.prev[:i:i], o.prev[i+1:]...)
			break
		}
	}
	return nil
}

// Order returns an order of the current nodes that satisfies every dependency
// and differs as little as possible from the previous output. The result is a
// fresh slice. The call rebuilds the whole arrangement from scratch.
func (o *Orderer) Order() []int {
	o.stats.Rebuilds++
	o.stats.Allocs++

	ids := make([]int, 0, len(o.nodes))
	for id, ok := range o.nodes {
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)

	pos := make(map[int]int, len(o.prev))
	for i, id := range o.prev {
		pos[id] = i
	}
	indeg := make(map[int]int, len(ids))
	for _, id := range ids {
		indeg[id] = len(o.pred[id])
	}

	h := &nodeHeap{o: o}
	for _, id := range ids {
		if indeg[id] == 0 {
			h.push(hItem{pos: pos[id], id: id})
		}
	}

	res := make([]int, 0, len(ids))
	for h.len() > 0 {
		it := h.pop()
		res = append(res, it.id)
		for _, v := range o.succ[it.id] {
			o.stats.Compares++
			indeg[v]--
			if indeg[v] == 0 {
				h.push(hItem{pos: pos[v], id: v})
			}
		}
	}

	o.prev = append([]int(nil), res...)
	o.stats.Allocs++
	return append([]int(nil), res...)
}

// Stats returns a snapshot of the internal work counters.
func (o *Orderer) Stats() Stats { return o.stats }

func (o *Orderer) hasCycle() bool {
	indeg := make(map[int]int, len(o.nodes))
	ids := make([]int, 0, len(o.nodes))
	for id, ok := range o.nodes {
		if ok {
			ids = append(ids, id)
			indeg[id] = len(o.pred[id])
		}
	}
	sort.Ints(ids)

	q := make([]int, 0, len(ids))
	for _, id := range ids {
		if indeg[id] == 0 {
			q = append(q, id)
		}
	}
	done := 0
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		done++
		for _, v := range o.succ[u] {
			o.stats.Compares++
			indeg[v]--
			if indeg[v] == 0 {
				q = append(q, v)
			}
		}
	}
	return done != len(ids)
}

func removeInt(s []int, v int) []int {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// hItem is a node queued for ordering, keyed by its position in the previous
// output and then by its id so the choice is fully deterministic.
type hItem struct{ pos, id int }

type nodeHeap struct {
	items []hItem
	o     *Orderer
}

func (h *nodeHeap) len() int { return len(h.items) }

func (h *nodeHeap) less(i, j int) bool {
	a, b := h.items[i], h.items[j]
	return h.o.cmpNodes(a.pos, a.id, b.pos, b.id) < 0
}

func (h *nodeHeap) push(it hItem) {
	h.items = append(h.items, it)
	i := len(h.items) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !h.less(i, p) {
			break
		}
		h.items[i], h.items[p] = h.items[p], h.items[i]
		i = p
	}
}

func (h *nodeHeap) pop() hItem {
	n := len(h.items)
	top := h.items[0]
	h.items[0] = h.items[n-1]
	h.items = h.items[:n-1]
	n--
	i := 0
	for {
		l, r := 2*i+1, 2*i+2
		small := i
		if l < n && h.less(l, small) {
			small = l
		}
		if r < n && h.less(r, small) {
			small = r
		}
		if small == i {
			break
		}
		h.items[i], h.items[small] = h.items[small], h.items[i]
		i = small
	}
	return top
}

// cmpNodes compares two nodes by their previous position, then by id. It counts
// the comparison as a unit of work.
func (o *Orderer) cmpNodes(posA, idA, posB, idB int) int {
	o.stats.Compares++
	if posA != posB {
		if posA < posB {
			return -1
		}
		return 1
	}
	if idA < idB {
		return -1
	}
	if idA > idB {
		return 1
	}
	return 0
}
