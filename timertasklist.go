package main

import (
	"sync/atomic"
)

type timerTaskList struct {
	root        *timerTaskEntry
	expiration  atomic.Int64
	taskCounter *atomic.Int64
}

func newTimerTaskList(taskCounter *atomic.Int64) *timerTaskList {
	root := &timerTaskEntry{}
	root.next = root
	root.prev = root
	l := &timerTaskList{
		root:        root,
		taskCounter: taskCounter,
	}
	l.expiration.Store(-1)
	return l
}

func (l *timerTaskList) GetDelay() int64 {
	return l.getExpiration()
}

// Get the bucket's expiration time
func (t *timerTaskList) getExpiration() int64 {
	return t.expiration.Load()
}

func (t *timerTaskList) add(timerTaskEntry *timerTaskEntry) {
	var done = false
	for !done {
		// Remove the timer task entry if it is already in any other list
		// We do this outside of the sync block below to avoid deadlocking.
		// We may retry until timerTaskEntry.list becomes null.
		timerTaskEntry.remove()

		//   synchronized {
		// timerTaskEntry.synchronized {
		if timerTaskEntry.list == nil {
			// put the timer task entry to the end of the list. (root.prev points to the tail entry)
			tail := t.root.prev
			timerTaskEntry.next = t.root
			timerTaskEntry.prev = tail
			timerTaskEntry.list = t
			tail.next = timerTaskEntry
			t.root.prev = timerTaskEntry
			t.taskCounter.Add(1)
			done = true
		}
	}
}

// Remove the specified timer task entry from this list
func (t *timerTaskList) remove(timerTaskEntry *timerTaskEntry) {
	// TODO: synchronized {
	//   timerTaskEntry.synchronized {
	if timerTaskEntry.list == t {
		timerTaskEntry.next.prev = timerTaskEntry.prev
		timerTaskEntry.prev.next = timerTaskEntry.next
		timerTaskEntry.next = nil
		timerTaskEntry.prev = nil
		timerTaskEntry.list = nil
		t.taskCounter.Add(-1)
	}
}

// Remove all task entries and apply the supplied function to each of them
func (t *timerTaskList) flush(f func(timerTaskEntry *timerTaskEntry)) {
	// synchronized {
	var head = t.root.next
	for head != t.root {
		t.remove(head)
		f(head)
		head = t.root.next
	}
	t.expiration.Store(-1)
}

// Set the bucket's expiration time
// Returns true if the expiration time is changed
func (t *timerTaskList) setExpiration(expirationMs int64) bool {
	return t.expiration.Swap(expirationMs) != expirationMs
}

type timerTaskEntry struct {
	list       *timerTaskList
	next, prev *timerTaskEntry

	timerTask    *TimerTask
	expirationMs int64 // 过期时间戳
}

func newTimerTaskEntry(timerTask *TimerTask, expirationMs int64) *timerTaskEntry {
	entry := &timerTaskEntry{
		timerTask:    timerTask,
		expirationMs: expirationMs,
	}
	if timerTask != nil {
		timerTask.setTimerTaskEntry(entry)
	}
	return entry
}

func (e *timerTaskEntry) cancelled() bool {
	return e.timerTask.getTimerTaskEntry() != e
}

func (e *timerTaskEntry) remove() {
	var currentList = e.list
	// If remove is called when another thread is moving the entry from a task entry list to another,
	// this may fail to remove the entry due to the change of value of list. Thus, we retry until the list becomes null.
	// In a rare case, this thread sees null and exits the loop, but the other thread insert the entry to another list later.
	for currentList != nil {
		currentList.remove(e)
		currentList = e.list
	}
}
