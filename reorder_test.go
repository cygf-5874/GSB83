package minreorder

import "testing"

func mustNew(t *testing.T, ids []int, deps [][2]int) *Orderer {
	t.Helper()
	o, err := NewOrderer(ids, deps)
	if err != nil {
		t.Fatalf("NewOrderer(%v, %v) unexpected error: %v", ids, deps, err)
	}
	return o
}

func eq(t *testing.T, name string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length = %d, want %d (got %v, want %v)", name, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: element %d = %d, want %d (got %v, want %v)", name, i, got[i], want[i], got, want)
		}
	}
}

func TestEmptyOrderer(t *testing.T) {
	o := mustNew(t, nil, nil)
	if got := o.Order(); len(got) != 0 {
		t.Fatalf("empty orderer: Order() = %v, want empty", got)
	}
}

func TestNoDepsKeepsGivenOrder(t *testing.T) {
	o := mustNew(t, []int{5, 3, 9, 1}, nil)
	eq(t, "no deps", o.Order(), []int{5, 3, 9, 1})
}

func TestChainReorder(t *testing.T) {
	o := mustNew(t, []int{3, 2, 1}, [][2]int{{1, 2}, {2, 3}})
	eq(t, "chain", o.Order(), []int{1, 2, 3})
}

func TestMinimalMove(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, [][2]int{{3, 1}})
	eq(t, "minimal move", o.Order(), []int{2, 3, 1})
}

func TestLexicographicTieBreak(t *testing.T) {
	o := mustNew(t, []int{2, 1, 3}, [][2]int{{3, 1}})
	eq(t, "lex tie", o.Order(), []int{2, 3, 1})
}

func TestInsertAfterLast(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, nil)
	if err := o.Insert(4, []int{3}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	eq(t, "insert after last", o.Order(), []int{1, 2, 3, 4})
}

func TestInsertWithoutDeps(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, nil)
	if err := o.Insert(4, nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	eq(t, "insert without deps", o.Order(), []int{1, 2, 3, 4})
}

func TestInsertUnknownDep(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, nil)
	before := o.Order()
	if err := o.Insert(4, []int{99}); err != ErrBadDep {
		t.Fatalf("Insert with unknown dep: err = %v, want ErrBadDep", err)
	}
	eq(t, "state unchanged after bad dep", o.Order(), before)
}

func TestInsertSelfCycle(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, nil)
	before := o.Order()
	if err := o.Insert(4, []int{4}); err != ErrCycle {
		t.Fatalf("Insert self cycle: err = %v, want ErrCycle", err)
	}
	eq(t, "state unchanged after cycle", o.Order(), before)
}

func TestConstructorCycle(t *testing.T) {
	if _, err := NewOrderer([]int{1, 2, 3}, [][2]int{{1, 2}, {2, 3}, {3, 1}}); err != ErrCycle {
		t.Fatalf("cyclic constructor: err = %v, want ErrCycle", err)
	}
}

func TestConstructorUnknownDep(t *testing.T) {
	if _, err := NewOrderer([]int{1, 2}, [][2]int{{1, 9}}); err != ErrBadDep {
		t.Fatalf("unknown dep constructor: err = %v, want ErrBadDep", err)
	}
}

func TestInsertDeletedDep(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, nil)
	if err := o.Delete(2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := o.Insert(4, []int{2}); err != ErrBadDep {
		t.Fatalf("Insert after deleted node: err = %v, want ErrBadDep", err)
	}
}

func TestDeleteKeepsValidOrder(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, [][2]int{{1, 2}})
	if err := o.Delete(2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	eq(t, "after delete", o.Order(), []int{1, 3})
}

func TestOrderIsRepeatable(t *testing.T) {
	o := mustNew(t, []int{4, 1, 3, 2}, [][2]int{{1, 2}, {2, 3}})
	first := o.Order()
	for i := 0; i < 50; i++ {
		eq(t, "repeat", o.Order(), first)
	}
}

func TestOrderIndependentOfDepListingOrder(t *testing.T) {
	a := mustNew(t, []int{4, 1, 3, 2}, [][2]int{{1, 2}, {2, 3}})
	b := mustNew(t, []int{4, 1, 3, 2}, [][2]int{{2, 3}, {1, 2}})
	eq(t, "dep listing order", a.Order(), b.Order())
}

func TestStatsTrackWork(t *testing.T) {
	o := mustNew(t, []int{1, 2, 3}, [][2]int{{3, 1}})
	eq(t, "before stats", o.Order(), []int{2, 3, 1})
	eq(t, "after stats", o.Order(), []int{2, 3, 1})
	s := o.Stats()
	if s.Compares <= 0 {
		t.Fatalf("Compares = %d, want > 0", s.Compares)
	}
	if s.Allocs <= 0 {
		t.Fatalf("Allocs = %d, want > 0", s.Allocs)
	}
	if s.Rebuilds < 0 {
		t.Fatalf("Rebuilds = %d, want >= 0", s.Rebuilds)
	}
}
