Redis Bloom filters
-------------
[![Test](https://github.com/HoangViet144/bloom/actions/workflows/test.yml/badge.svg)](https://github.com/HoangViet144/bloom/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/HoangViet144/bloom.svg)](https://pkg.go.dev/github.com/HoangViet144/bloom)

A Bloom filter is a concise/compressed representation of a set, where the main
requirement is to make membership queries; _i.e._, whether an item is a
member of a set. A Bloom filter will always correctly report the presence
of an element in the set when the element is indeed present. A Bloom filter 
can use much less storage than the original set, but it allows for some 'false positives':
it may sometimes report that an element is in the set whereas it is not.

When you construct, you need to know how many elements you have (the desired capacity), and what is the desired false positive rate you are willing to tolerate. A common false-positive rate is 1%. The
lower the false-positive rate, the more memory you are going to require. Similarly, the higher the
capacity, the more memory you will use.
You may construct the Bloom filter capable of receiving 1 million elements with a false-positive
rate of 1% in the following manner. 

```Go
    redisClient := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{":6379"}})
    bitset := bloom.NewRedisBitSet(redisClient, uuid.New().String(), time.Minute)
    filter := bloom.NewWithEstimates(1000000, 0.01, bitset) 
```

You should call `NewWithEstimates` conservatively: if you specify a number of elements that it is
too small, the false-positive bound might be exceeded. A Bloom filter is not a dynamic data structure:
you must know ahead of time what your desired capacity is.

Our implementation accepts keys for setting and testing as `[]byte`. Thus, to
add a string item, `"Love"`:

```Go
    filter.Add([]byte("Love"))
```

Similarly, to test if `"Love"` is in bloom:

```Go
    if filter.Test([]byte("Love"))
```

For numerical data, we recommend that you look into the encoding/binary library. But, for example, to add a `uint32` to the filter:

```Go
    i := uint32(100)
    n1 := make([]byte, 4)
    binary.BigEndian.PutUint32(n1, i)
    filter.Add(n1)
```

Godoc documentation:  https://pkg.go.dev/github.com/HoangViet144/bloom

## Installation

```bash
go get github.com/HoangViet144/bloom
```

## Verifying the False Positive Rate


Sometimes, the actual false positive rate may differ (slightly) from the
theoretical false positive rate. We have a function to estimate the false positive rate of a
Bloom filter with _m_ bits and _k_ hashing functions for a set of size _n_:

```Go
    if bloom.EstimateFalsePositiveRate(20*n, 5, n) > 0.001 ...
```

You can use it to validate the computed m, k parameters:

```Go
    m, k := bloom.EstimateParameters(n, fp)
    ActualfpRate := bloom.EstimateFalsePositiveRate(m, k, n, bitset)
```

or

```Go
    f := bloom.NewWithEstimates(n, fp, bitset)
    ActualfpRate := bloom.EstimateFalsePositiveRate(f.m, f.k, n, bitset)
```

You would expect `ActualfpRate` to be close to the desired false-positive rate `fp` in these cases.

The `EstimateFalsePositiveRate` function creates a temporary Bloom filter. It is
also relatively expensive and only meant for validation.


## Contributing

If you wish to contribute to this project, please branch and issue a pull request against master ("[GitHub Flow](https://guides.github.com/introduction/flow/)")

This project includes a Makefile that allows you to test and build the project with simple commands.
To see all available options:
```bash
make help
```

## Design

A Bloom filter has two parameters: _m_, the number of bits used in storage, and _k_, the number of hashing functions on elements of the set. (The actual hashing functions are important, too, but this is not a parameter for this implementation). A Bloom filter is backed by a [BitSet](https://github.com/bits-and-blooms/bitset); a key is represented in the filter by setting the bits at each value of the  hashing functions (modulo _m_). Set membership is done by _testing_ whether the bits at each value of the hashing functions (again, modulo _m_) are set. If so, the item is in the set. If the item is actually in the set, a Bloom filter will never fail (the true positive rate is 1.0); but it is susceptible to false positives. The art is to choose _k_ and _m_ correctly.

In this implementation, the hashing functions used is [murmurhash](github.com/twmb/murmur3), a non-cryptographic hashing function.


Given the particular hashing scheme, it's best to be empirical about this. Note
that estimating the FP rate will clear the Bloom filter.
