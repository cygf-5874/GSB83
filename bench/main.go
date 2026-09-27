// Command bench reports the cost of the minreorder library at a few scales.
//
// For each scale it builds an initial DAG, performs a fixed number of
// insertions (each followed by an Order call) and prints the resulting
// Stats together with the wall-clock time. The wall-clock number is only
// informational; the acceptance criterion is the Stats counters.
package main

import (
	"fmt"
	"time"

	"minreorder"
)

func buildDeps(n int) [][2]int {
	var deps [][2]int
	for i := 1; i < n; i++ {
		for j := 1; j <= 11; j++ {
			if i-j >= 0 {
				deps = append(deps, [2]int{i - j, i})
			}
		}
	}
	return deps
}

func run(initial, inserts int) {
	ids := make([]int, initial)
	for i := range ids {
		ids[i] = i
	}
	o, err := minreorder.NewOrderer(ids, buildDeps(initial))
	if err != nil {
		fmt.Printf("scale %d/%d: NewOrderer failed: %v\n", initial, inserts, err)
		return
	}

	start := time.Now()
	next := initial
	for i := 0; i < inserts; i++ {
		if err := o.Insert(next, []int{next - 1}); err != nil {
			fmt.Printf("scale %d/%d: Insert failed: %v\n", initial, inserts, err)
			return
		}
		o.Order()
		next++
	}
	elapsed := time.Since(start)

	s := o.Stats()
	fmt.Printf("%d nodes / %d inserts: Rebuilds=%d Compares=%d Allocs=%d elapsed=%s\n",
		initial+inserts, inserts, s.Rebuilds, s.Compares, s.Allocs, elapsed.Round(time.Millisecond))
}

func main() {
	run(1000, 500)
	run(5000, 2500)
	run(10000, 10000)
}
