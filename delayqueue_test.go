package main

import (
	"sync/atomic"
	"testing"
)

func TestPriorityQueue_Len(t *testing.T) {
	task := &timerTaskList{
		root:        nil,
		expiration:  atomic.Int64{},
		taskCounter: nil,
	}
	task.setExpiration(1)
	dq := &DelayQueue{}
	dq.Offer(task)
	exitC := make(chan struct{})
	excute := dq.Poll(exitC, 2)
	if excute == nil {
		t.Error("xxx")
	}
}
