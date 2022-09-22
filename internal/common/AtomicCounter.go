package common

import (
	"errors"
	"sync"
)

type AtomicCounter struct {
	// Implemented in this fashion to allow multi-statement locks during critical section.
	mutex    sync.Mutex
	isLocked bool

	Count int32
}

func (ac *AtomicCounter) AtomicAdd(delta int32) (int32, error) {
	if !ac.isLocked {
		return -1, errors.New("lock required to mutate atomic counter")
	}
	ac.Count += delta
	return ac.Count, nil

}

func (ac *AtomicCounter) AtomicRead() (int32, error) {
	if !ac.isLocked {
		return -1, errors.New("lock required to read atomic counter")
	}
	return ac.Count, nil
}

func (ac *AtomicCounter) Lock() {
	ac.mutex.Lock()
	ac.isLocked = true
}

func (ac *AtomicCounter) Unlock() {
	ac.isLocked = false
	ac.mutex.Unlock()
}
