package main

type TimerTask struct {
	delayMs        int64
	timerTaskEntry *timerTaskEntry
	f              func()
}

func (t *TimerTask) setTimerTaskEntry(entry *timerTaskEntry) {
	// TODO: synchronized {
	// if this timerTask is already held by an existing timer task entry,
	// we will remove such an entry first.
	if t.timerTaskEntry != nil && t.timerTaskEntry != entry {
		t.timerTaskEntry.remove()
	}
	t.timerTaskEntry = entry
}

func (t *TimerTask) getTimerTaskEntry() *timerTaskEntry {
	return t.timerTaskEntry
}
