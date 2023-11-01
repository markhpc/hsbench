package main

import (
	"fmt"
	"io"
	"math/rand"
)

var _ io.ReadSeeker = &RandomReadSeeker{}

var ErrNotInitialized = fmt.Errorf("not initialized")

type RandomReadSeeker struct {
	seed int64
	src  io.Reader

	pos    int64
	length int64
}

func NewRandomReadSeeker(seed, length int64) *RandomReadSeeker {
	r := &RandomReadSeeker{
		length: length,
		seed:   seed,
	}

	r.resetSrc()

	return r
}

func (r *RandomReadSeeker) isInitialized() bool {
	return r != nil && r.seed != 0 && r.src != nil
}

func (r *RandomReadSeeker) resetSrc() {
	r.src = io.LimitReader(rand.New(rand.NewSource(r.seed)), r.length)
}

func (r *RandomReadSeeker) Seek(offset int64, whence int) (int64, error) {
	if !r.isInitialized() {
		return 0, ErrNotInitialized
	}

	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.pos + offset
	case io.SeekEnd:
		abs = r.length + offset
	default:
		return 0, fmt.Errorf("unknown whence %d", whence)
	}

	if abs < 0 {
		return 0, fmt.Errorf("negative position")
	}

	if abs == r.pos {
		return abs, nil
	} else if abs > r.pos && abs < r.length {
		_, err := io.CopyN(io.Discard, r.src, abs-r.pos)
		if err != nil {
			return 0, err
		}
	} else if abs < r.length {
		r.resetSrc()
		_, err := io.CopyN(io.Discard, r.src, abs)
		if err != nil {
			return 0, err
		}
	}

	r.pos = abs

	return r.pos, nil
}

func (r *RandomReadSeeker) Read(p []byte) (n int, err error) {
	if !r.isInitialized() {
		return 0, ErrNotInitialized
	}

	if r.pos >= r.length {
		return 0, io.EOF
	}

	n, err = r.src.Read(p)

	r.pos += int64(n)

	return
}
