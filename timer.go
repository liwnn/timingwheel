package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type taskExecutor struct {
}

func (e taskExecutor) submit(timerTask *TimerTask) {
	timerTask.f()
}

type SystemTimer struct {
	taskExecutor taskExecutor

	delayQueue  *DelayQueue
	taskCounter atomic.Int64
	timingWheel *TimingWheel

	lock  sync.RWMutex
	exitC chan struct{}
}

func NewSystemTimer(tickMs int64, wheelSize int64, startMs int64) *SystemTimer {
	t := &SystemTimer{
		delayQueue: NewDelayQueue(),
		exitC:      make(chan struct{}),
	}
	t.timingWheel = NewTimingWheel(tickMs, wheelSize, startMs, &t.taskCounter, t.delayQueue)
	return t
}

func (st *SystemTimer) add(timerTask *TimerTask) {
	st.lock.RLock() // 读锁！！!
	defer st.lock.RUnlock()
	st.addTimerTaskEntry(newTimerTaskEntry(timerTask, timerTask.delayMs*int64(time.Millisecond)+time.Now().UnixMilli()))
}

func (st *SystemTimer) AfterFunc(ms int64, f func()) {
	st.add(&TimerTask{
		delayMs: ms,
		f:       f,
	})
}

func (st *SystemTimer) addTimerTaskEntry(timerTaskEntry *timerTaskEntry) {
	if !st.timingWheel.add(timerTaskEntry) {
		// Already expired or cancelled
		if !timerTaskEntry.cancelled() {
			st.taskExecutor.submit(timerTaskEntry.timerTask)
		}
	}
}

// advanceClock advances the clock if there is an expired bucket. If there isn't any expired bucket when called,
// waits up to timeoutMs before giving up.
func (st *SystemTimer) advanceClock(timeoutMs int64) bool {
	var bucket = st.delayQueue.Poll(st.exitC, timeoutMs)
	if bucket != nil {
		st.lock.Lock()
		defer st.lock.Unlock()
		for bucket != nil {
			st.timingWheel.advanceClock(bucket.getExpiration())
			bucket.flush(st.addTimerTaskEntry)
			bucket = st.delayQueue.Poll(st.exitC, timeoutMs)
		}
		return true
	} else {
		return false
	}
}

func (st *SystemTimer) Run() {
	for {
		select {
		case <-st.exitC:
			return
		default:
			st.advanceClock(time.Now().UnixMilli() + 100)
		}
	}
}

func (st *SystemTimer) Shutdown() {
	close(st.exitC)
}

func (st *SystemTimer) Size() int {
	return int(st.taskCounter.Load())
}
