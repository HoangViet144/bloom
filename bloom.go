/*
Package bloom provides data structures and methods for creating Bloom filters.

A Bloom filter is a representation of a set of _n_ items, where the main
requirement is to make membership queries; _i.e._, whether an item is a
member of a set.

A Bloom filter has two parameters: _m_, a maximum size (typically a reasonably large
multiple of the cardinality of the set to represent) and _k_, the number of hashing
functions on elements of the set. (The actual hashing functions are important, too,
but this is not a parameter for this implementation). A Bloom filter is backed by
a BitSet; a key is represented in the filter by setting the bits at each value of the
hashing functions (modulo _m_). Set membership is done by _testing_ whether the
bits at each value of the hashing functions (again, modulo _m_) are set. If so,
the item is in the set. If the item is actually in the set, a Bloom filter will
never fail (the true positive rate is 1.0); but it is susceptible to false
positives. The art is to choose _k_ and _m_ correctly.

This filter accepts keys for setting as testing as []byte. Thus, to
add a string item, "Love":

    uint n = 1000
    filter := bloom.New(20*n, 5) // load of 20, 5 keys
    filter.Add([]byte("Love"))

Similarly, to test if "Love" is in bloom:

    if filter.Test([]byte("Love"))

For numeric data, I recommend that you look into the binary/encoding library. But,
for example, to add an uint32 to the filter:

    i := uint32(100)
    n1 := make([]byte,4)
    binary.BigEndian.PutUint32(n1,i)
    f.Add(n1)

Finally, there is a method to estimate the false positive rate of a
Bloom filter with _m_ bits and _k_ hashing functions for a set of size _n_:

    if bloom.EstimateFalsePositiveRate(20*n, 5, n) > 0.001 ...

You can use it to validate the computed m, k parameters:

    m, k := bloom.EstimateParameters(n, fp)
    ActualfpRate := bloom.EstimateFalsePositiveRate(m, k, n)

or

	f := bloom.NewWithEstimates(n, fp)
	ActualfpRate := bloom.EstimateFalsePositiveRate(f.m, f.k, n)

You would expect ActualfpRate to be close to the desired fp in these cases.

The EstimateFalsePositiveRate function creates a temporary Bloom filter. It is
also relatively expensive and only meant for validation.
*/

package bloom

import (
	"encoding/binary"
	"io"
	"math"
)

type BloomFilter interface {
	// Cap returns the capacity, _m_, of a Bloom filter
	Cap() uint
	// K returns the number of hash functions used in the BloomFilter
	K() uint
	// BitSet returns the underlying bitset for this filter.
	BitSet() BitSet
	// Add data to the Bloom Filter. Returns the filter (allows chaining)
	Add(data []byte) BloomFilter
	// AddString to the Bloom Filter. Returns the filter (allows chaining)
	AddString(data string) BloomFilter
	// Test returns true if the data is in the BloomFilter, false otherwise.
	// If true, the result might be a false positive. If false, the data
	// is definitely not in the set.
	Test(data []byte) bool
	// TestString returns true if the string is in the BloomFilter, false otherwise.
	// If true, the result might be a false positive. If false, the data
	// is definitely not in the set.
	TestString(data string) bool
	// TestLocations returns true if all locations are set in the BloomFilter, false
	// otherwise.
	TestLocations(locs []uint64) bool
	// TestAndAdd is the equivalent to calling Test(data) then Add(data).
	// Returns the result of Test.
	TestAndAdd(data []byte) bool
	// TestAndAddString is the equivalent to calling Test(string) then Add(string).
	// Returns the result of Test.
	TestAndAddString(data string) bool
	// TestOrAdd is the equivalent to calling Test(data) then if not present Add(data).
	// Returns the result of Test.
	TestOrAdd(data []byte) bool
	// TestOrAddString is the equivalent to calling Test(string) then if not present Add(string).
	// Returns the result of Test.
	TestOrAddString(data string) bool
	// ClearAll clears all the data in a Bloom filter, removing all keys
	ClearAll() BloomFilter
	// ApproximatedSize approximates the number of items
	// https://en.wikipedia.org/wiki/Bloom_filter#Approximating_the_number_of_items_in_a_Bloom_filter
	ApproximatedSize() uint32
	// MarshalJSON implements json.Marshaler interface.
	MarshalJSON() ([]byte, error)
	// UnmarshalJSON implements json.Unmarshaler interface.
	UnmarshalJSON(data []byte) error
	// WriteTo writes a binary representation of the BloomFilter to an i/o stream.
	// It returns the number of bytes written.
	WriteTo(stream io.Writer) (int64, error)
	// ReadFrom reads a binary representation of the BloomFilter (such as might
	// have been written by WriteTo()) from an i/o stream. It returns the number
	// of bytes read.
	ReadFrom(stream io.Reader) (int64, error)
	// GobEncode implements gob.GobEncoder interface.
	GobEncode() ([]byte, error)
	// GobDecode implements gob.GobDecoder interface.
	GobDecode(data []byte) error
	// Equal tests for the equality of two Bloom filters
	Equal(g BloomFilter) bool
}

// New creates a new Bloom filter with _m_ bits and _k_ hashing functions
// We force _m_ and _k_ to be at least one to avoid panics.
func New(m uint, k uint, b BitSet) BloomFilter {
	return &bloomFilterImpl{
		m: max(1, m),
		k: max(1, k),
		b: b.Init(m),
	}
}

// From creates a new Bloom filter with len(_data_) * 64 bits and _k_ hashing
// functions. The data slice is not going to be reset.
func From(data []uint64, k uint, b BitSet) BloomFilter {
	m := uint(len(data) * 64)
	return FromWithM(data, m, k, b)
}

// FromWithM creates a new Bloom filter with _m_ length, _k_ hashing functions.
// The data slice is not going to be reset.
func FromWithM(data []uint64, m, k uint, b BitSet) BloomFilter {
	return &bloomFilterImpl{m, k, b.From(data)}
}

// EstimateParameters estimates requirements for m and k.
// Based on https://bitbucket.org/ww/bloom/src/829aa19d01d9/bloom.go
// used with permission.
func EstimateParameters(n uint, p float64) (m uint, k uint) {
	m = uint(math.Ceil(-1 * float64(n) * math.Log(p) / math.Pow(math.Log(2), 2)))
	k = uint(math.Ceil(math.Log(2) * float64(m) / float64(n)))
	return
}

// NewWithEstimates creates a new Bloom filter for about n items with fp
// false positive rate
func NewWithEstimates(n uint, fp float64, b BitSet) BloomFilter {
	m, k := EstimateParameters(n, fp)
	return New(m, k, b)
}

// EstimateFalsePositiveRate returns, for a BloomFilter of m bits
// and k hash functions, an estimation of the false positive rate when
//
//	storing n entries. This is an empirical, relatively slow
//
// test using integers as keys.
// This function is useful to validate the implementation.
func EstimateFalsePositiveRate(m, k, n uint, b BitSet) (fpRate float64) {
	rounds := uint32(100000)
	// We construct a new filter.
	f := New(m, k, b)
	n1 := make([]byte, 4)
	// We populate the filter with n values.
	for i := uint32(0); i < uint32(n); i++ {
		binary.BigEndian.PutUint32(n1, i)
		f.Add(n1)
	}
	fp := 0
	// test for number of rounds
	for i := uint32(0); i < rounds; i++ {
		binary.BigEndian.PutUint32(n1, i+uint32(n)+1)
		if f.Test(n1) {
			fp++
		}
	}
	fpRate = float64(fp) / (float64(rounds))
	return
}
