package main

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"
)

// An Item is something we manage in a priority queue.
type Item struct {
	value    Delayed // The value of the item; arbitrary.
	priority int64   // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq PriorityQueue) Peek() *Item {
	if pq.Len() == 0 {
		return nil
	}
	return pq[0]
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, value Delayed) {
	item.value = value
	item.priority = value.GetDelay()
	heap.Fix(pq, item.index)
}

func (pq *PriorityQueue) poll() *Item {
	x := heap.Pop(pq)
	return x.(*Item)
}

type Delayed interface {
	GetDelay() int64
}

type DelayQueue struct {
	lock sync.RWMutex
	q    PriorityQueue

	sleeping int32
	wakeupC  chan struct{}
}

func NewDelayQueue() *DelayQueue {
	return &DelayQueue{
		wakeupC: make(chan struct{}),
	}
}

func (dq *DelayQueue) Offer(e Delayed) {
	item := &Item{
		value:    e,
		priority: e.GetDelay(),
		index:    0,
	}
	dq.lock.Lock()
	heap.Push(&dq.q, item)
	index := item.index
	dq.lock.Unlock()
	if index == 0 {
		if atomic.CompareAndSwapInt32(&dq.sleeping, 1, 0) {
			// 通知等地协程
			dq.wakeupC <- struct{}{}
		}
	}
}

func (dq *DelayQueue) Poll(exitC <-chan struct{}, timeoutMs int64) *timerTaskList {
	defer atomic.StoreInt32(&dq.sleeping, 0)
	if timeoutMs < time.Now().UnixMilli() {
		timeoutMs = time.Now().UnixMilli()
	}
	for {
		dq.lock.Lock()
		first := dq.q.Peek()
		if first == nil {
			dq.lock.Unlock()
			delay := timeoutMs - time.Now().UnixMilli()
			if delay <= 0 {
				return nil
			}
			atomic.StoreInt32(&dq.sleeping, 1)
			select {
			case <-exitC:
				return nil
			case <-dq.wakeupC:
				break
			case <-time.After(time.Millisecond * time.Duration(delay)):
			}
			atomic.StoreInt32(&dq.sleeping, 0)
		} else {
			var t = first.value.GetDelay()
			if t > timeoutMs {
				t = timeoutMs
			}
			delay := t - time.Now().UnixMilli()
			if delay <= 0 {
				item := dq.q.poll()
				dq.lock.Unlock()
				return item.value.(*timerTaskList)
			}
			dq.lock.Unlock()
			atomic.StoreInt32(&dq.sleeping, 1)
			select {
			case <-exitC:
				return nil
			case <-dq.wakeupC:
				break
			case <-time.After(time.Millisecond * time.Duration(delay)):
			}
			atomic.StoreInt32(&dq.sleeping, 0)
		}
	}
}
