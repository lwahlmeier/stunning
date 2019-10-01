package easyatomics // import "github.com/lwahlmeier/easyatomics"

import (
	"sync/atomic"
)

type AtomicUint64 struct {
	val uint64
}

func (at *AtomicUint64) Inc() uint64 {
	return atomic.AddUint64(&at.val, 1)
}

func (at *AtomicUint64) Dec() uint64 {
	return atomic.AddUint64(&at.val, ^uint64(0))
}

func (at *AtomicUint64) IncBy(i uint64) uint64 {
	return atomic.AddUint64(&at.val, i)
}

func (at *AtomicUint64) DecBy(i uint64) uint64 {
	return atomic.AddUint64(&at.val, ^uint64(i-1))
}

func (at *AtomicUint64) Set(i uint64) uint64 {
	atomic.StoreUint64(&at.val, i)
	return i
}

func (at *AtomicUint64) Get() uint64 {
	return atomic.LoadUint64(&at.val)
}

type AtomicInt64 struct {
	val int64
}

func (at *AtomicInt64) Inc() int64 {
	return atomic.AddInt64(&at.val, 1)
}

func (at *AtomicInt64) Dec() int64 {
	return atomic.AddInt64(&at.val, -1)
}

func (at *AtomicInt64) IncBy(i int64) int64 {
	return atomic.AddInt64(&at.val, i)
}

func (at *AtomicInt64) DecBy(i int64) int64 {
	return atomic.AddInt64(&at.val, -i)
}

func (at *AtomicInt64) Set(i int64) int64 {
	atomic.StoreInt64(&at.val, i)
	return i
}

func (at *AtomicInt64) Get() int64 {
	return atomic.LoadInt64(&at.val)
}

type AtomicUint32 struct {
	val uint32
}

func (at *AtomicUint32) Inc() uint32 {
	return atomic.AddUint32(&at.val, 1)
}

func (at *AtomicUint32) Dec() uint32 {
	return atomic.AddUint32(&at.val, ^uint32(0))
}

func (at *AtomicUint32) IncBy(i uint32) uint32 {
	return atomic.AddUint32(&at.val, i)
}

func (at *AtomicUint32) DecBy(i uint32) uint32 {
	return atomic.AddUint32(&at.val, ^uint32(i-1))
}

func (at *AtomicUint32) Set(i uint32) uint32 {
	atomic.StoreUint32(&at.val, i)
	return i
}

func (at *AtomicUint32) Get() uint32 {
	return atomic.LoadUint32(&at.val)
}

type AtomicInt32 struct {
	val int32
}

func (at *AtomicInt32) Inc() int32 {
	return atomic.AddInt32(&at.val, 1)
}

func (at *AtomicInt32) Dec() int32 {
	return atomic.AddInt32(&at.val, -1)
}

func (at *AtomicInt32) IncBy(i int32) int32 {
	return atomic.AddInt32(&at.val, i)
}

func (at *AtomicInt32) DecBy(i int32) int32 {
	return atomic.AddInt32(&at.val, -i)
}

func (at *AtomicInt32) Set(i int32) int32 {
	atomic.StoreInt32(&at.val, i)
	return i
}

func (at *AtomicInt32) Get() int32 {
	return atomic.LoadInt32(&at.val)
}
