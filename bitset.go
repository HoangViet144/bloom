package bloom

import "io"

type BitSet interface {
	// Init allocate bit set based on bit length
	Init(length uint) BitSet
	// Set bit i to 1, the capacity of the bitset is automatically
	// increased accordingly.
	// If i>= Cap(), this function will panic.
	// Warning: using a very large value for 'i'
	// may lead to a memory shortage and a panic: the caller is responsible
	// for providing sensible parameters in line with their memory capacity.
	Set(i uint) BitSet
	// UnSet bit i to 0
	UnSet(i uint) BitSet
	// InPlaceUnion creates the destructive union of base set and compare set.
	// This is the BitSet equivalent of | (or).
	InPlaceUnion(compare BitSet)
	// Test whether bit i is set.
	Test(i uint) bool
	// ClearAll clears the entire BitSet
	ClearAll() BitSet
	// Count (number of set bits).
	// Also known as "popcount" or "population count".
	Count() uint
	// WriteTo writes a BitSet to a stream
	WriteTo(stream io.Writer) (int64, error)
	// Equal tests the equivalence of two BitSets.
	// False if they are of different sizes, otherwise true
	// only if all the same bits are set
	Equal(c BitSet) bool
	// GetBitSetKey returns the key of redis bitset. It is used only for redis bitset
	GetBitSetKey() string
	// ReadFrom reads a BitSet from a stream written using WriteTo
	ReadFrom(stream io.Reader) (int64, error)
	// From is a constructor used to create a BitSet from an array of integers
	From(buf []uint64) BitSet
}
