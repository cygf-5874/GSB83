// Command check is the fixed acceptance program for the minreorder task.
//
// It runs ten scenarios across four groups and prints one line per scenario:
//
//	PASS <group>/<name>
//	FAIL <group>/<name>  expected=... actual=...
//
// and a final "结果：通过 x/N" line. It exits 0 only when every scenario
// passes, and never stops early on the first failure. It supports "-list" to
// print the scenario names and "--only <group>" to run a single group.
//
// The scenarios judge cost with the Stats counters only (Rebuilds and
// Compares); wall-clock time is never used.
package main

import (
	"fmt"
	"os"
	"strings"

	"minreorder"
)

type scenario struct {
	group string
	name  string
	run   func() (string, string) // returns (expected, actual); equal means pass
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fmtInts(a []int) string {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// ---------------------------------------------------------------- semantics

func semLCSMax() (string, string) {
	o, err := minreorder.NewOrderer([]int{1, 2, 3, 4, 5}, [][2]int{{3, 1}, {5, 2}})
	if err != nil {
		return "[3 1 4 5 2]", "NewOrderer error: " + err.Error()
	}
	return fmtInts([]int{3, 1, 4, 5, 2}), fmtInts(o.Order())
}

func semLexTie() (string, string) {
	o, err := minreorder.NewOrderer([]int{2, 1, 3}, [][2]int{{3, 1}})
	if err != nil {
		return "[2 3 1]", "NewOrderer error: " + err.Error()
	}
	return fmtInts([]int{2, 3, 1}), fmtInts(o.Order())
}

func semCycleNoChange() (string, string) {
	if _, err := minreorder.NewOrderer([]int{1, 2, 3}, [][2]int{{1, 2}, {2, 3}, {3, 1}}); err != minreorder.ErrCycle {
		return "NewOrderer->ErrCycle", fmt.Sprintf("NewOrderer->%v", err)
	}
	o, err := minreorder.NewOrderer([]int{1, 2, 3}, nil)
	if err != nil {
		return "ErrCycle, state unchanged", "NewOrderer error: " + err.Error()
	}
	before := o.Order()
	if err := o.Insert(4, []int{4}); err != minreorder.ErrCycle {
		return fmt.Sprintf("Insert->ErrCycle, order=%s", fmtInts(before)),
			fmt.Sprintf("Insert->%v", err)
	}
	after := o.Order()
	if !eqInts(before, after) {
		return fmt.Sprintf("order=%s", fmtInts(before)), fmt.Sprintf("order=%s", fmtInts(after))
	}
	return "cycle rejected, state unchanged", "cycle rejected, state unchanged"
}

func semBadDep() (string, string) {
	o, err := minreorder.NewOrderer([]int{1, 2, 3}, nil)
	if err != nil {
		return "ErrBadDep", "NewOrderer error: " + err.Error()
	}
	if err := o.Insert(4, []int{99}); err != minreorder.ErrBadDep {
		return "Insert unknown -> ErrBadDep", fmt.Sprintf("Insert unknown -> %v", err)
	}
	if err := o.Delete(2); err != nil {
		return "Delete ok", "Delete error: " + err.Error()
	}
	if err := o.Insert(5, []int{2}); err != minreorder.ErrBadDep {
		return "Insert removed -> ErrBadDep", fmt.Sprintf("Insert removed -> %v", err)
	}
	return "bad deps rejected", "bad deps rejected"
}

// -------------------------------------------------------------- determinism

// eqSeq is a fixed operation sequence used by both the determinism and the
// equivalence scenarios.
func runSeq() (*minreorder.Orderer, []int) {
	o, _ := minreorder.NewOrderer([]int{2, 1, 3}, [][2]int{{3, 1}})
	out := o.Order()
	o.Insert(4, []int{3})
	out = o.Order()
	o.Insert(5, []int{4})
	out = o.Order()
	o.Insert(6, nil)
	out = o.Order()
	o.Delete(3)
	out = o.Order()
	o.Insert(7, []int{5})
	out = o.Order()
	return o, out
}

func detRepeat() (string, string) {
	o1, out1 := runSeq()
	o2, out2 := runSeq()
	if !eqInts(out1, out2) {
		return fmtInts(out1), fmtInts(out2)
	}
	if o1.Stats().Rebuilds != 0 || o2.Stats().Rebuilds != 0 {
		return "Rebuilds=0", fmt.Sprintf("Rebuilds=%d,%d", o1.Stats().Rebuilds, o2.Stats().Rebuilds)
	}
	return "identical output, Rebuilds=0", "identical output, Rebuilds=0"
}

func detShuffledDeps() (string, string) {
	a, errA := minreorder.NewOrderer([]int{4, 3, 2, 1}, [][2]int{{4, 3}, {2, 1}})
	b, errB := minreorder.NewOrderer([]int{4, 3, 2, 1}, [][2]int{{2, 1}, {4, 3}})
	if errA != nil || errB != nil {
		return "no error", fmt.Sprintf("errors: %v %v", errA, errB)
	}
	oa, ob := a.Order(), b.Order()
	if !eqInts(oa, ob) {
		return fmtInts(oa), fmtInts(ob)
	}
	if a.Stats().Rebuilds != 0 || b.Stats().Rebuilds != 0 {
		return "Rebuilds=0", fmt.Sprintf("Rebuilds=%d,%d", a.Stats().Rebuilds, b.Stats().Rebuilds)
	}
	return "same output, Rebuilds=0", "same output, Rebuilds=0"
}

// ------------------------------------------------------------------ budget

type costResult struct {
	nodes, edges, lowerBound int64
	stats                    minreorder.Stats
}

var costCache *costResult

func buildCost() *costResult {
	if costCache != nil {
		return costCache
	}
	const initial, inserts = 4000, 2000
	ids := make([]int, initial)
	for i := range ids {
		ids[i] = i
	}
	var deps [][2]int
	for i := 1; i < initial; i++ {
		for j := 1; j <= 8; j++ {
			if i-j >= 0 {
				deps = append(deps, [2]int{i - j, i})
			}
		}
	}
	edges := int64(len(deps))
	o, err := minreorder.NewOrderer(ids, deps)
	if err != nil {
		panic("cost scenario: " + err.Error())
	}
	var lowerBound int64
	next := initial
	for i := 0; i < inserts; i++ {
		after := []int{next - 1}
		if err := o.Insert(next, after); err != nil {
			panic("cost scenario insert: " + err.Error())
		}
		lowerBound += int64(len(after))
		edges += int64(len(after))
		o.Order()
		next++
	}
	costCache = &costResult{
		nodes:      int64(initial + inserts),
		edges:      edges,
		lowerBound: lowerBound,
		stats:      o.Stats(),
	}
	return costCache
}

func budgetNoRebuild() (string, string) {
	c := buildCost()
	if c.stats.Rebuilds != 0 {
		return "Rebuilds=0", fmt.Sprintf("Rebuilds=%d", c.stats.Rebuilds)
	}
	return "Rebuilds=0", "Rebuilds=0"
}

func budgetCompareBound() (string, string) {
	c := buildCost()
	bound := int64(6) * (c.nodes + c.edges)
	if c.stats.Compares > bound {
		return fmt.Sprintf("Compares<=%d", bound), fmt.Sprintf("Compares=%d", c.stats.Compares)
	}
	got := fmt.Sprintf("Compares=%d, bound=%d", c.stats.Compares, bound)
	return got, got
}

func budgetStatsHonest() (string, string) {
	c := buildCost()
	if c.stats.Compares < c.lowerBound {
		return fmt.Sprintf("Compares>=%d", c.lowerBound), fmt.Sprintf("Compares=%d", c.stats.Compares)
	}
	if c.stats.Rebuilds != 0 {
		return "Rebuilds=0", fmt.Sprintf("Rebuilds=%d", c.stats.Rebuilds)
	}
	got := fmt.Sprintf("Compares=%d in [%d, %d], Rebuilds=0", c.stats.Compares, c.lowerBound, int64(6)*(c.nodes+c.edges))
	return got, got
}

// ------------------------------------------------------------- equivalence

func equivStable() (string, string) {
	o, out := runSeq()
	want := []int{2, 1, 4, 5, 6, 7}
	if !eqInts(out, want) {
		return fmtInts(want), fmtInts(out)
	}
	if r := o.Stats().Rebuilds; r != 0 {
		return fmt.Sprintf("order=%s, Rebuilds=0", fmtInts(want)), fmt.Sprintf("order=%s, Rebuilds=%d", fmtInts(out), r)
	}
	return fmt.Sprintf("order=%s, Rebuilds=0", fmtInts(want)), fmt.Sprintf("order=%s, Rebuilds=0", fmtInts(out))
}

func scenarios() []scenario {
	return []scenario{
		{"semantics", "lcs_max", semLCSMax},
		{"semantics", "lex_tie", semLexTie},
		{"semantics", "cycle_no_change", semCycleNoChange},
		{"semantics", "bad_dep", semBadDep},
		{"determinism", "repeat", detRepeat},
		{"determinism", "shuffled_deps", detShuffledDeps},
		{"budget", "no_rebuild", budgetNoRebuild},
		{"budget", "compare_bound", budgetCompareBound},
		{"budget", "stats_honest", budgetStatsHonest},
		{"equivalence", "stable_output", equivStable},
	}
}

func main() {
	list := false
	only := ""
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch {
		case a == "-list":
			list = true
		case a == "--only":
			if i+1 < len(os.Args) {
				only = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--only="):
			only = strings.TrimPrefix(a, "--only=")
		}
	}

	all := scenarios()
	if list {
		for _, s := range all {
			fmt.Printf("%s/%s\n", s.group, s.name)
		}
		return
	}

	passed, total := 0, 0
	for _, s := range all {
		if only != "" && s.group != only {
			continue
		}
		total++
		exp, act := s.run()
		if exp == act {
			passed++
			fmt.Printf("PASS %s/%s\n", s.group, s.name)
		} else {
			fmt.Printf("FAIL %s/%s  期望=%s 实际=%s\n", s.group, s.name, exp, act)
		}
	}
	fmt.Printf("结果：通过 %d/%d\n", passed, total)
	if passed != total {
		os.Exit(1)
	}
}
