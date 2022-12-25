package bloom

import (
	"context"
	"encoding/binary"
	"io"
	"time"

	"github.com/go-redis/redis/v9"
)

func NewRedisBitSet(redisClient redis.UniversalClient, bitsetKey string, expiration time.Duration) BitSet {
	return &RedisBitSet{
		redisClient: redisClient,
		bitsetKey: bitsetKey,
		expiration: expiration,
	}
}

type RedisBitSet struct {
	redisClient redis.UniversalClient
	bitsetKey   string
	expiration  time.Duration
}

func (r *RedisBitSet)Init(length uint) BitSet  {
	r.UnSet(length)
	return r
}

func (r *RedisBitSet) UnSet(i uint) BitSet {
	r.redisClient.SetBit(context.Background(), r.bitsetKey, int64(i), 0)
	return r
}

func (r *RedisBitSet) Set(i uint) BitSet {
	r.redisClient.SetBit(context.Background(), r.bitsetKey, int64(i), 1)
	return r
}

func (r *RedisBitSet) InPlaceUnion(compare BitSet) {
	r.redisClient.BitOpOr(context.Background(), r.bitsetKey, compare.GetBitSetKey())
}

func (r *RedisBitSet) Test(i uint) bool {
	return r.redisClient.GetBit(context.Background(), r.bitsetKey, int64(i)).Val() == 1
}

func (r *RedisBitSet) ClearAll() BitSet {
	r.redisClient.Set(context.Background(), r.bitsetKey, "", r.expiration)
	return r
}

func (r *RedisBitSet) Count() uint {
	return uint(r.redisClient.BitCount(context.Background(), r.bitsetKey, nil).Val())
}

func (r *RedisBitSet) WriteTo(stream io.Writer) (int64, error) {
	err := binary.Write(stream, binary.BigEndian, uint64(len(r.bitsetKey)))
	if err != nil {
		return 0, err
	}
	n, err := stream.Write([]byte(r.bitsetKey))
	err = binary.Write(stream, binary.BigEndian, uint64(r.expiration))
	if err != nil {
		return 0, err
	}
	bitsetVal := []byte(r.redisClient.Get(context.Background(), r.bitsetKey).Val())
	err = binary.Write(stream, binary.BigEndian, uint64(len(bitsetVal)))
	if err != nil {
		return 0, err
	}
	m, err :=stream.Write(bitsetVal)
	return int64(n + m + 3*binary.Size(uint64(0))), err
}

func (r *RedisBitSet) Equal(c BitSet) bool {
	return r.redisClient.Get(context.Background(), r.bitsetKey).Val() == r.redisClient.Get(context.Background(), c.GetBitSetKey()).Val()
}

func (r *RedisBitSet) GetBitSetKey() string {
	return r.bitsetKey
}

func (r *RedisBitSet) ReadFrom(stream io.Reader) (int64, error) {
	var bitsetKeyLen, expiration, bitsetValLen uint64
	err := binary.Read(stream, binary.BigEndian, &bitsetKeyLen)
	if err != nil {
		return 0, err
	}

	bitsetKeyBytes := make([]byte, bitsetKeyLen, bitsetKeyLen)
	n, err := stream.Read(bitsetKeyBytes)
	if err != nil {
		return 0, err
	}
	r.bitsetKey = string(bitsetKeyBytes)

	err = binary.Read(stream, binary.BigEndian, &expiration)
	if err != nil {
		return 0, err
	}
	r.expiration = time.Duration(expiration)

	err = binary.Read(stream, binary.BigEndian, &bitsetValLen)
	if err != nil {
		return 0, err
	}
	data := make([]byte, bitsetValLen, bitsetValLen)
	m, err := stream.Read(data)
	if err != nil {
		return 0, err
	}

	r.redisClient.Set(context.Background(), r.bitsetKey, data, r.expiration)

	return int64(n + m + 3*binary.Size(uint64(0))), nil
}

func (r *RedisBitSet) From(buf []uint64) BitSet {
	byteAr := make([]byte,0,0)
	for _, val := range buf {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, val)
		byteAr = append(byteAr, b...)
	}

	r.redisClient.Set(context.Background(), r.bitsetKey, string(byteAr), r.expiration)
	return r
}
