package peerlimit

import (
	"maps"
	"reflect"
	"testing"
)

const (
	Node1 = "Node1"
	Node2 = "Node2"
)

type gcounterTest struct {
	name string
	recv gCounter
	in   gCounter
	want gCounter
}

type crdtTest struct {
	name string
	recv crdt
	in   crdt
	want crdt
}

var gcTests = []gcounterTest{
	{
		name: "new key added",
		recv: gCounter{Node1: 7},
		in:   gCounter{Node2: 10},
		want: gCounter{Node1: 7, Node2: 10},
	},
	{
		name: "max value taken",
		recv: gCounter{Node1: 7},
		in:   gCounter{Node1: 10},
		want: gCounter{Node1: 10},
	},
	{
		name: "max value taken mirrored",
		recv: gCounter{Node1: 10},
		in:   gCounter{Node1: 7},
		want: gCounter{Node1: 10},
	},
}

var crdtTests = []crdtTest{
	{
		name: "new key into empty crdt",
		recv: crdt{},
		in:   crdt{user2ID: gCounter{Node1: 10}},
		want: crdt{user2ID: gCounter{Node1: 10}},
	},
	{
		name: "shared key merges nested gCounter",
		recv: crdt{userID: gCounter{Node1: 10}},
		in:   crdt{userID: gCounter{Node2: 5}},
		want: crdt{userID: gCounter{Node1: 10, Node2: 5}},
	},
	{
		name: "foreign key does not clobber own keys",
		recv: crdt{userID: gCounter{Node1: 10}},
		in:   crdt{user2ID: gCounter{Node2: 5}},
		want: crdt{userID: gCounter{Node1: 10}, user2ID: gCounter{Node2: 5}},
	},
}

// Mechanics of gCounter.merge: new cells are added and existing cells take the
// max, so a smaller incoming value never clobbers a larger one.
func TestMerge_GCounter(t *testing.T) {
	for _, tt := range gcTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.recv.merge(tt.in)
			if !maps.Equal(tt.recv, tt.want) {
				t.Fatal("the maps should be equal")
			}
		})
	}
}

// Mechanics of crdt.merge: a new key is created without a nil-map panic, a
// shared key merges its nested gCounter, and a foreign key leaves own keys intact.
func TestMerge_CRDT(t *testing.T) {
	for _, tt := range crdtTests {
		t.Run(tt.name, func(t *testing.T) {
			tt.recv.merge(tt.in)
			if !reflect.DeepEqual(tt.recv, tt.want) {
				t.Fatal("the maps should be equal")
			}
		})
	}
}

// deepCopy returns an independent copy of c: the outer map is cloned and each
// nested gCounter is cloned too. maps.Clone alone is shallow — it would copy
// the outer map but leave the nested gCounters shared by reference, so a merge
// on the copy would also mutate the original. Property tests below rely on this
// because merge mutates its receiver in place.
func deepCopy(c crdt) crdt {
	out := make(crdt, len(c))
	for key, gc := range c {
		out[key] = maps.Clone(gc)
	}
	return out
}

// Property fixtures overlap on both keys and nodes so that max actually has
// something to choose between.
var (
	propA = crdt{userID: gCounter{Node1: 10, Node2: 3}}
	propB = crdt{userID: gCounter{Node1: 4, Node2: 7}, user2ID: gCounter{Node1: 2}}
	propC = crdt{userID: gCounter{Node1: 8}, user2ID: gCounter{Node2: 9}}
)

// Idempotent: merging the same state twice equals merging it once. This is the
// property that lets gossip redeliver/reorder messages without double-counting.
func TestMerge_CRDT_Idempotent(t *testing.T) {
	once := deepCopy(propA)
	once.merge(propB)

	twice := deepCopy(propA)
	twice.merge(propB)
	twice.merge(propB)

	if !reflect.DeepEqual(once, twice) {
		t.Fatalf("merge is not idempotent:\nonce  = %v\ntwice = %v", once, twice)
	}
}

// Commutative: a∪b == b∪a. Order in which peers gossip to each other must not
// matter to the converged state.
func TestMerge_CRDT_Commutative(t *testing.T) {
	left := deepCopy(propA)
	left.merge(propB)

	right := deepCopy(propB)
	right.merge(propA)

	if !reflect.DeepEqual(left, right) {
		t.Fatalf("merge is not commutative:\nleft  = %v\nright = %v", left, right)
	}
}

// Associative: (a∪b)∪c == a∪(b∪c). Grouping of merges across the cluster must
// not matter to the converged state.
func TestMerge_CRDT_Associative(t *testing.T) {
	left := deepCopy(propA)
	left.merge(propB)
	left.merge(propC)

	bc := deepCopy(propB)
	bc.merge(propC)
	right := deepCopy(propA)
	right.merge(bc)

	if !reflect.DeepEqual(left, right) {
		t.Fatalf("merge is not associative:\nleft  = %v\nright = %v", left, right)
	}
}
