/*
In this implementation, the hashing functions used is murmurhash,
a non-cryptographic hashing function.
*/
package bloom

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
)

// A BloomFilter is a representation of a set of _n_ items, where the main
// requirement is to make membership queries; _i.e._, whether an item is a
// member of a set.
type bloomFilterImpl struct {
	m uint
	k uint
	b BitSet
}

// location returns the ith hashed location using the four base hash values
func (f *bloomFilterImpl) location(h [4]uint64, i uint) uint {
	return uint(location(h, i) % uint64(f.m))
}

func (f *bloomFilterImpl) Cap() uint {
	return f.m
}

func (f *bloomFilterImpl) K() uint {
	return f.k
}

func (f *bloomFilterImpl) BitSet() BitSet {
	return f.b
}

func (f *bloomFilterImpl) Add(data []byte) BloomFilter {
	h := baseHashes(data)
	for i := uint(0); i < f.k; i++ {
		f.b.Set(f.location(h, i))
	}
	return f
}

func (f *bloomFilterImpl) AddString(data string) BloomFilter {
	return f.Add([]byte(data))
}

func (f *bloomFilterImpl) Test(data []byte) bool {
	h := baseHashes(data)
	for i := uint(0); i < f.k; i++ {
		if !f.b.Test(f.location(h, i)) {
			return false
		}
	}
	return true
}

func (f *bloomFilterImpl) TestString(data string) bool {
	return f.Test([]byte(data))
}

func (f *bloomFilterImpl) TestLocations(locs []uint64) bool {
	for i := 0; i < len(locs); i++ {
		if !f.b.Test(uint(locs[i] % uint64(f.m))) {
			return false
		}
	}
	return true
}

func (f *bloomFilterImpl) TestAndAdd(data []byte) bool {
	present := true
	h := baseHashes(data)
	for i := uint(0); i < f.k; i++ {
		l := f.location(h, i)
		if !f.b.Test(l) {
			present = false
		}
		f.b.Set(l)
	}
	return present
}

func (f *bloomFilterImpl) TestAndAddString(data string) bool {
	return f.TestAndAdd([]byte(data))
}

func (f *bloomFilterImpl) TestOrAdd(data []byte) bool {
	present := true
	h := baseHashes(data)
	for i := uint(0); i < f.k; i++ {
		l := f.location(h, i)
		if !f.b.Test(l) {
			present = false
			f.b.Set(l)
		}
	}
	return present
}

func (f *bloomFilterImpl) TestOrAddString(data string) bool {
	return f.TestOrAdd([]byte(data))
}

func (f *bloomFilterImpl) ClearAll() BloomFilter {
	f.b.ClearAll()
	return f
}

func (f *bloomFilterImpl) ApproximatedSize() uint32 {
	x := float64(f.b.Count())
	m := float64(f.Cap())
	k := float64(f.K())
	size := -1 * m / k * math.Log(1-x/m) / math.Log(math.E)
	return uint32(math.Floor(size + 0.5)) // round
}

// bloomFilterJSON is an unexported type for marshaling/unmarshaling BloomFilter struct.
type bloomFilterJSON struct {
	M uint   `json:"m"`
	K uint   `json:"k"`
	B BitSet `json:"b"`
}

func (f *bloomFilterImpl) MarshalJSON() ([]byte, error) {
	return json.Marshal(bloomFilterJSON{f.m, f.k, f.b})
}

func (f *bloomFilterImpl) UnmarshalJSON(data []byte) error {
	var j bloomFilterJSON
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	f.m = j.M
	f.k = j.K
	f.b = j.B
	return nil
}

func (f *bloomFilterImpl) WriteTo(stream io.Writer) (int64, error) {
	err := binary.Write(stream, binary.BigEndian, uint64(f.m))
	if err != nil {
		return 0, err
	}
	err = binary.Write(stream, binary.BigEndian, uint64(f.k))
	if err != nil {
		return 0, err
	}
	numBytes, err := f.b.WriteTo(stream)
	return numBytes + int64(2*binary.Size(uint64(0))), err
}

func (f *bloomFilterImpl) ReadFrom(stream io.Reader) (int64, error) {
	var m, k uint64
	err := binary.Read(stream, binary.BigEndian, &m)
	if err != nil {
		return 0, err
	}
	err = binary.Read(stream, binary.BigEndian, &k)
	if err != nil {
		return 0, err
	}
	numBytes, err := f.b.ReadFrom(stream)
	if err != nil {
		return 0, err
	}
	f.m = uint(m)
	f.k = uint(k)
	return numBytes + int64(2*binary.Size(uint64(0))), nil
}

func (f *bloomFilterImpl) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (f *bloomFilterImpl) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	_, err := f.ReadFrom(buf)

	return err
}

func (f *bloomFilterImpl) Equal(g BloomFilter) bool {
	return f.m == g.Cap() && f.k == g.K() && f.b.Equal(g.BitSet())
}
