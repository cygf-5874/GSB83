package minreorder

// Stats reports the internal work an Orderer has performed since it was
// created. The field names and their meaning are part of the public
// contract and must not be changed.
type Stats struct {
	// Compares is the number of pairwise node comparisons performed while
	// deciding dependency order (dependency checks plus the comparisons the
	// ordering structure itself performs to pick the next node).
	Compares int64
	// Allocs is the number of heap allocations attributed to the library.
	Allocs int64
	// Rebuilds is the number of times the whole arrangement was rebuilt
	// from scratch. This is the metric the task asks to drive down.
	Rebuilds int64
}
