package kawasaki

import (
	"strconv"
	"sync/atomic"
)

type idgen struct {
	next int64
}

func NewSequentialIDGenerator(seed int64) IDGenerator {
	return &idgen{
		next: seed,
	}
}

func (s *idgen) Generate() string {
	next := atomic.AddInt64(&s.next, 1)
	containerID := []byte{}

	var i uint
	for i = 0; i < 11; i++ {
		shift := 55 - (i+1)*5
		character := (next >> shift) & 31
		containerID = strconv.AppendInt(containerID, character, 32)
	}

	return string(containerID)
}
