package main

import (
	"sync/atomic"
)

type TimingWheel struct {
	tickMs        int64            // 一个slot的时间
	wheelSize     int64            // 有多少个slot
	interval      int64            // 总时间跨度
	currentTime   int64            // 当前时间
	buckets       []*timerTaskList // 所有槽的数据
	taskCounter   *atomic.Int64    // 该时间轮任务总数
	queue         *DelayQueue      // 延时队列
	overflowWheel *TimingWheel     // 溢出轮
}

func NewTimingWheel(tickMs int64, wheelSize int64, startMs int64, taskCounter *atomic.Int64, delayQueue *DelayQueue) *TimingWheel {
	buckets := make([]*timerTaskList, wheelSize)
	for i := range buckets {
		buckets[i] = newTimerTaskList(taskCounter)
	}
	return &TimingWheel{
		tickMs:      tickMs,
		wheelSize:   wheelSize,
		interval:    tickMs * wheelSize,
		currentTime: startMs - (startMs % tickMs),
		buckets:     buckets,
		taskCounter: taskCounter,
		queue:       delayQueue,
	}
}

func (tw *TimingWheel) add(timerTaskEntry *timerTaskEntry) bool {
	expiration := timerTaskEntry.expirationMs

	if timerTaskEntry.cancelled() {
		// Cancelled
		return false
	} else if expiration < tw.currentTime+tw.tickMs {
		// Already expired
		return false
	} else if expiration < tw.currentTime+tw.interval {
		// Put in its own bucket
		virtualId := expiration / tw.tickMs
		bucket := tw.buckets[virtualId%tw.wheelSize]
		bucket.add(timerTaskEntry)

		// Set the bucket expiration time
		if bucket.setExpiration(virtualId * tw.tickMs) {
			// The bucket needs to be enqueued because it was an expired bucket
			// We only need to enqueue the bucket when its expiration time has changed, i.e. the wheel has advanced
			// and the previous buckets gets reused; further calls to set the expiration within the same wheel cycle
			// will pass in the same value and hence return false, thus the bucket with the same expiration will not
			// be enqueued multiple times.
			tw.queue.Offer(bucket)
		}
		return true
	} else {
		// Out of the interval. Put it into the parent timer
		if tw.overflowWheel == nil {
			tw.addOverflowWheel()
		}
		return tw.overflowWheel.add(timerTaskEntry)
	}
}

// Try to advance the clock
func (tw *TimingWheel) advanceClock(timeMs int64) {
	if timeMs >= tw.currentTime+tw.tickMs {
		tw.currentTime = timeMs - (timeMs % tw.tickMs)

		// Try to advance the clock of the overflow wheel if present
		if tw.overflowWheel != nil {
			tw.overflowWheel.advanceClock(tw.currentTime)
		}
	}
}

func (tw *TimingWheel) addOverflowWheel() {
	if tw.overflowWheel == nil {
		tw.overflowWheel = NewTimingWheel(tw.interval, tw.wheelSize, tw.currentTime, tw.taskCounter, tw.queue)
	}
}
