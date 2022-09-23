package main

import (
	"fmt"
	"testing"
	"time"
)

func TestTimer(t *testing.T) {
	s := NewSystemTimer(1, 10, time.Now().Unix())
	go s.Run()
	defer s.Shutdown()

	exitC := make(chan time.Time, 1)
	s.AfterFunc(1, func() {
		fmt.Println("The timer fires")
		exitC <- time.Now()
	})

	<-exitC
}

func TestTimerStop(t *testing.T) {
	s := NewSystemTimer(1, 10, time.Now().Unix())
	go s.Run()

	go func() {
		time.Sleep(10 * time.Millisecond)
		s.AfterFunc(1, func() {
			fmt.Println("The timer fires")
		})
	}()
	s.AfterFunc(200, func() {
		fmt.Println("The timer fires")
	})
	<-time.After(100 * time.Millisecond)
	// Stop the timer before it fires
	s.Shutdown()
}
