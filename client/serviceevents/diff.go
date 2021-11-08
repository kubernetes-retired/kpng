package serviceevents

// TODO can be extracted as a generic package if needed

type Diff struct {
	SameKey func(prevIdx, currIdx int) bool
	Added   func(currIdx int)
	Updated func(prevIdx, currIdx int)
	Deleted func(prevIdx int)
}

func (d Diff) SlicesLen(prevLen, currLen int) {
prevLoop:
	for i := 0; i < prevLen; i++ {
		for j := 0; j < currLen; j++ {
			if d.SameKey(i, j) {
				d.Updated(i, j)
				continue prevLoop
			}
		}

		// previous value not found in current values
		d.Deleted(i)
	}

currLoop:
	for j := 0; j < currLen; j++ {
		for i := 0; i < prevLen; i++ {
			if d.SameKey(i, j) {
				continue currLoop
			}
		}

		// current value not found in previous values
		d.Added(j)
	}
}
