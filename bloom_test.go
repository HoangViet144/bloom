package bloom

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
)

//This implementation of Bloom filters is _not_
//safe for concurrent use. Uncomment the following
//method and run go test -race

func TestConcurrent(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	gmp := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(gmp)

	f := New(1000, 4, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	n1 := []byte("Bess")
	n2 := []byte("Jane")
	f.Add(n1)
	f.Add(n2)

	var wg sync.WaitGroup
	const try = 1000
	var err1, err2 error

	wg.Add(1)
	go func() {
		for i := 0; i < try; i++ {
			n1b := f.Test(n1)
			if !n1b {
				err1 = fmt.Errorf("%v should be in", n1)
				break
			}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < try; i++ {
			n2b := f.Test(n2)
			if !n2b {
				err2 = fmt.Errorf("%v should be in", n2)
				break
			}
		}
		wg.Done()
	}()

	wg.Wait()

	if err1 != nil {
		t.Fatal(err1)
	}
	if err2 != nil {
		t.Fatal(err2)
	}
}

func TestBasic(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)

	f := New(1000, 4, redisBitSet)
	n1 := []byte("Bess")
	n2 := []byte("Jane")
	n3 := []byte("Emma")
	f.Add(n1)
	n3a := f.TestAndAdd(n3)
	n1b := f.Test(n1)
	n2b := f.Test(n2)
	n3b := f.Test(n3)
	if !n1b {
		t.Errorf("%v should be in.", n1)
	}
	if n2b {
		t.Errorf("%v should not be in.", n2)
	}
	if n3a {
		t.Errorf("%v should not be in the first time we look.", n3)
	}
	if !n3b {
		t.Errorf("%v should be in the second time we look.", n3)
	}
}

func TestBasicUint32(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	f := New(1000, 4, redisBitSet)
	n1 := make([]byte, 4)
	n2 := make([]byte, 4)
	n3 := make([]byte, 4)
	n4 := make([]byte, 4)
	n5 := make([]byte, 4)
	binary.BigEndian.PutUint32(n1, 100)
	binary.BigEndian.PutUint32(n2, 101)
	binary.BigEndian.PutUint32(n3, 102)
	binary.BigEndian.PutUint32(n4, 103)
	binary.BigEndian.PutUint32(n5, 104)
	f.Add(n1)
	n3a := f.TestAndAdd(n3)
	n1b := f.Test(n1)
	n2b := f.Test(n2)
	n3b := f.Test(n3)
	n5a := f.TestOrAdd(n5)
	n5b := f.Test(n5)
	f.Test(n4)
	if !n1b {
		t.Errorf("%v should be in.", n1)
	}
	if n2b {
		t.Errorf("%v should not be in.", n2)
	}
	if n3a {
		t.Errorf("%v should not be in the first time we look.", n3)
	}
	if !n3b {
		t.Errorf("%v should be in the second time we look.", n3)
	}
	if n5a {
		t.Errorf("%v should not be in the first time we look.", n5)
	}
	if !n5b {
		t.Errorf("%v should be in the second time we look.", n5)
	}
}

func TestNewWithLowNumbers(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	f := New(0, 0, redisBitSet)
	if f.K() != 1 {
		t.Errorf("%v should be 1", f.K())
	}
	if f.Cap() != 1 {
		t.Errorf("%v should be 1", f.Cap())
	}
}

func TestString(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	f := NewWithEstimates(1000, 0.001, redisBitSet)
	n1 := "Love"
	n2 := "is"
	n3 := "in"
	n4 := "bloom"
	n5 := "blooms"
	f.AddString(n1)
	n3a := f.TestAndAddString(n3)
	n1b := f.TestString(n1)
	n2b := f.TestString(n2)
	n3b := f.TestString(n3)
	n5a := f.TestOrAddString(n5)
	n5b := f.TestString(n5)
	f.TestString(n4)
	if !n1b {
		t.Errorf("%v should be in.", n1)
	}
	if n2b {
		t.Errorf("%v should not be in.", n2)
	}
	if n3a {
		t.Errorf("%v should not be in the first time we look.", n3)
	}
	if !n3b {
		t.Errorf("%v should be in the second time we look.", n3)
	}
	if n5a {
		t.Errorf("%v should not be in the first time we look.", n5)
	}
	if !n5b {
		t.Errorf("%v should be in the second time we look.", n5)
	}

}

func testEstimated(n uint, maxFp float64, t *testing.T) {
	m, k := EstimateParameters(n, maxFp)
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	fpRate := EstimateFalsePositiveRate(m, k, n, redisBitSet)
	if fpRate > 1.5*maxFp {
		t.Errorf("False positive rate too high: n: %v; m: %v; k: %v; maxFp: %f; fpRate: %f, fpRate/maxFp: %f", n, m, k, maxFp, fpRate, fpRate/maxFp)
	}
}

//func TestEstimated1000_0001(t *testing.T)   { testEstimated(1000, 0.000100, t) }
//func TestEstimated10000_0001(t *testing.T)  { testEstimated(10000, 0.000100, t) }
//func TestEstimated100000_0001(t *testing.T) { testEstimated(100000, 0.000100, t) }
//
//func TestEstimated1000_001(t *testing.T)   { testEstimated(1000, 0.001000, t) }
//func TestEstimated10000_001(t *testing.T)  { testEstimated(10000, 0.001000, t) }
//func TestEstimated100000_001(t *testing.T) { testEstimated(100000, 0.001000, t) }
//
//func TestEstimated1000_01(t *testing.T)   { testEstimated(1000, 0.010000, t) }
//func TestEstimated10000_01(t *testing.T)  { testEstimated(10000, 0.010000, t) }
//func TestEstimated100000_01(t *testing.T) { testEstimated(100000, 0.010000, t) }

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

// The following function courtesy of Nick @turgon
// This helper function ranges over the input data, applying the hashing
// which returns the bit locations to set in the filter.
// For each location, increment a counter for that bit address.
//
// If the Bloom Filter's location() method distributes locations uniformly
// at random, a property it should inherit from its hash function, then
// each bit location in the filter should end up with roughly the same
// number of hits.  Importantly, the value of k should not matter.
//
// Once the results are collected, we can run a chi squared goodness of fit
// test, comparing the result histogram with the uniform distribition.
// This yields a test statistic with degrees-of-freedom of m-1.
func chiTestBloom(m, k, rounds uint, elements [][]byte, bitset BitSet) (succeeds bool) {
	f := New(m, k, bitset)
	results := make([]uint, m)
	chi := make([]float64, m)

	for _, data := range elements {
		h := baseHashes(data)
		for i := uint(0); i < f.K(); i++ {
			results[uint(location(h, i)%uint64(f.Cap()))]++
		}
	}

	// Each element of results should contain the same value: k * rounds / m.
	// Let's run a chi-square goodness of fit and see how it fares.
	var chiStatistic float64
	e := float64(k*rounds) / float64(m)
	for i := uint(0); i < m; i++ {
		chi[i] = math.Pow(float64(results[i])-e, 2.0) / e
		chiStatistic += chi[i]
	}

	// this tests at significant level 0.005 up to 20 degrees of freedom
	table := [20]float64{
		7.879, 10.597, 12.838, 14.86, 16.75, 18.548, 20.278,
		21.955, 23.589, 25.188, 26.757, 28.3, 29.819, 31.319, 32.801, 34.267,
		35.718, 37.156, 38.582, 39.997}
	df := min(m-1, 20)

	succeeds = table[df-1] > chiStatistic
	return

}

func TestLocation(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	var m, k, rounds uint

	m = 8
	k = 3

	rounds = 100000 // 15000000

	elements := make([][]byte, rounds)

	for x := uint(0); x < rounds; x++ {
		ctrlist := make([]uint8, 4)
		ctrlist[0] = uint8(x)
		ctrlist[1] = uint8(x >> 8)
		ctrlist[2] = uint8(x >> 16)
		ctrlist[3] = uint8(x >> 24)
		data := []byte(ctrlist)
		elements[x] = data
	}

	succeeds := chiTestBloom(m, k, rounds, elements, redisBitSet)
	if !succeeds {
		t.Error("random assignment is too unrandom")
	}

}

func TestCap(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	f := New(1000, 4, redisBitSet)
	if f.Cap() != f.Cap() {
		t.Error("not accessing Cap() correctly")
	}
}

func TestK(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	f := New(1000, 4, redisBitSet)
	if f.K() != f.K() {
		t.Error("not accessing K() correctly")
	}
}

func TestWriteToReadFrom(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	redisBitSet := NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
	var b bytes.Buffer
	f := New(1000, 4, redisBitSet)
	_, err := f.WriteTo(&b)
	if err != nil {
		t.Fatal(err)
	}

	g := New(1000, 1, redisBitSet)
	_, err = g.ReadFrom(&b)
	if err != nil {
		t.Fatal(err)
	}
	if g.Cap() != f.Cap() {
		t.Error("invalid m value")
	}
	if g.K() != f.K() {
		t.Error("invalid k value")
	}
	if g.BitSet() == nil {
		t.Fatal("bitset is nil")
	}
	if !g.BitSet().Equal(f.BitSet()) {
		t.Error("bitsets are not equal")
	}

	g.Test([]byte(""))
}

func TestReadWriteBinary(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := New(1000, 4, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	var buf bytes.Buffer
	bytesWritten, err := f.WriteTo(&buf)
	if err != nil {
		t.Fatal(err.Error())
	}
	if bytesWritten != int64(buf.Len()) {
		t.Errorf("incorrect write length %d != %d", bytesWritten, buf.Len())
	}

	g := New(0, 0, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	bytesRead, err := g.ReadFrom(&buf)
	if err != nil {
		t.Fatal(err.Error())
	}
	if bytesRead != bytesWritten {
		t.Errorf("read unexpected number of bytes %d != %d", bytesRead, bytesWritten)
	}
	if g.Cap() != f.Cap() {
		t.Error("invalid m value")
	}
	if g.K() != f.K() {
		t.Error("invalid k value")
	}
	if g.BitSet() == nil {
		t.Fatal("bitset is nil")
	}
	if !g.BitSet().Equal(f.BitSet()) {
		t.Error("bitsets are not equal")
	}
}

func TestEncodeDecodeGob(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := New(1000, 4, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	f.Add([]byte("one"))
	f.Add([]byte("two"))
	f.Add([]byte("three"))
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(f)
	if err != nil {
		t.Fatal(err.Error())
	}

	g := New(0, 0, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	err = gob.NewDecoder(&buf).Decode(&g)
	if err != nil {
		t.Fatal(err.Error())
	}
	if g.Cap() != f.Cap() {
		t.Error("invalid m value")
	}
	if g.K() != f.K() {
		t.Error("invalid k value")
	}
	if g.BitSet() == nil {
		t.Fatal("bitset is nil")
	}
	if !g.BitSet().Equal(f.BitSet()) {
		t.Error("bitsets are not equal")
	}
	if !g.Test([]byte("three")) {
		t.Errorf("missing value 'three'")
	}
	if !g.Test([]byte("two")) {
		t.Errorf("missing value 'two'")
	}
	if !g.Test([]byte("one")) {
		t.Errorf("missing value 'one'")
	}
}

func TestEqual(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := New(1000, 4, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	f1 := New(1000, 4, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	g := New(1000, 20, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	h := New(10, 20, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	n1 := []byte("Bess")
	f1.Add(n1)
	if !f.Equal(f) {
		t.Errorf("%v should be equal to itself", f)
	}
	if f.Equal(f1) {
		t.Errorf("%v should not be equal to %v", f, f1)
	}
	if f.Equal(g) {
		t.Errorf("%v should not be equal to %v", f, g)
	}
	if f.Equal(h) {
		t.Errorf("%v should not be equal to %v", f, h)
	}
}

//func BenchmarkEstimated(b *testing.B) {
//	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
//	for n := uint(100000); n <= 100000; n *= 10 {
//		for fp := 0.1; fp >= 0.0001; fp /= 10.0 {
//			fmt.Println(n, fp)
//			f := NewWithEstimates(n, fp, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
//			EstimateFalsePositiveRate(f.Cap(), f.K(), n, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
//		}
//	}
//}

func BenchmarkSeparateTestAndAdd(b *testing.B) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := NewWithEstimates(uint(b.N), 0.0001, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	key := make([]byte, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.BigEndian.PutUint32(key, uint32(i))
		f.Test(key)
		f.Add(key)
	}
}

func BenchmarkCombinedTestAndAdd(b *testing.B) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := NewWithEstimates(uint(b.N), 0.0001, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	key := make([]byte, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		binary.BigEndian.PutUint32(key, uint32(i))
		f.TestAndAdd(key)
	}
}

func TestFrom(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	var (
		k    = uint(5)
		data = make([]uint64, 10)
		test = []byte("test")
	)

	bitSetKey := uuid.New().String()
	bf := From(data, k, NewRedisBitSet(redisClient, bitSetKey, time.Minute))
	if bf.K() != k {
		t.Errorf("Constant k does not match the expected value")
	}

	if bf.Cap() != uint(len(data)*64) {
		t.Errorf("Capacity does not match the expected value")
	}

	if bf.Test(test) {
		t.Errorf("Bloom filter should not contain the value")
	}

	bf.Add(test)
	if !bf.Test(test) {
		t.Errorf("Bloom filter should contain the value")
	}

	bitSetBytes, _ := redisClient.Get(context.Background(), bitSetKey).Bytes()
	cloneData := make([]uint64, 0)
	for i := 0; i < len(bitSetBytes); i += 8 {
		startInd := i
		endInd := i + 8
		if endInd > len(bitSetBytes) {
			endInd = len(bitSetBytes)
		}
		cloneData = append(cloneData, binary.LittleEndian.Uint64(bitSetBytes[startInd:endInd]))
	}

	bf = From(cloneData, k, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	if !bf.Test(test) {
		t.Errorf("Bloom filter should contain the value")
	}
}

func TestTestLocations(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := NewWithEstimates(1000, 0.001, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	n1 := []byte("Love")
	n2 := []byte("is")
	n3 := []byte("in")
	n4 := []byte("bloom")
	f.Add(n1)
	n3a := f.TestLocations(Locations(n3, f.K()))
	f.Add(n3)
	n1b := f.TestLocations(Locations(n1, f.K()))
	n2b := f.TestLocations(Locations(n2, f.K()))
	n3b := f.TestLocations(Locations(n3, f.K()))
	n4b := f.TestLocations(Locations(n4, f.K()))
	if !n1b {
		t.Errorf("%v should be in.", n1)
	}
	if n2b {
		t.Errorf("%v should not be in.", n2)
	}
	if n3a {
		t.Errorf("%v should not be in the first time we look.", n3)
	}
	if !n3b {
		t.Errorf("%v should be in the second time we look.", n3)
	}
	if n4b {
		t.Errorf("%v should be in.", n4)
	}
}

func TestApproximatedSize(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := NewWithEstimates(1000, 0.001, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	f.Add([]byte("Love"))
	f.Add([]byte("is"))
	f.Add([]byte("in"))
	f.Add([]byte("bloom"))
	size := f.ApproximatedSize()
	if size != 4 {
		t.Errorf("%d should equal 4.", size)
	}
}

func TestFPP(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
	f := NewWithEstimates(1000, 0.001, NewRedisBitSet(redisClient, uuid.New().String(), time.Minute))
	for i := uint32(0); i < 1000; i++ {
		n := make([]byte, 4)
		binary.BigEndian.PutUint32(n, i)
		f.Add(n)
	}
	count := 0

	for i := uint32(0); i < 1000; i++ {
		n := make([]byte, 4)
		binary.BigEndian.PutUint32(n, i+1000)
		if f.Test(n) {
			count += 1
		}
	}
	if float64(count)/1000.0 > 0.001 {
		t.Errorf("Excessive fpp")
	}
}
