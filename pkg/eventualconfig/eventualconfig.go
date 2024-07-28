package eventualconfig

import (
	"errors"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"sync"
)

type Key int

type EventualConfig interface {
	GetValue(key Key) (<-chan any, error)
	SetValue(key Key, value any) error
}

func NewEventualConfig(keys ...Key) EventualConfig {
	out := &eventualConfig{
		m:  make(map[Key]chan any, len(keys)),
		mu: new(sync.RWMutex),
	}

	for _, key := range keys {
		out.m[key] = make(chan any)
	}

	return out
}

type eventualConfig struct {
	m  map[Key]chan any
	mu *sync.RWMutex
}

var (
	ErrCannotGetValue      = errors.New("cannot get value")
	ErrValueMustBeDeclared = errors.New("value must be declared at initialization")
)

func (ec *eventualConfig) GetValue(key Key) (<-chan any, error) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if ec.m[key] == nil {
		return nil, flaterrors.Join(ErrValueMustBeDeclared, ErrCannotGetValue)
	}

	return ec.m[key], nil
}

var (
	ErrCannotSetValue = errors.New("cannot set value")
)

func (ec *eventualConfig) SetValue(key Key, value any) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.m[key] == nil {
		return flaterrors.Join(ErrValueMustBeDeclared, ErrCannotSetValue)
	}

	go func() {
		for {
			ec.m[key] <- value
		}
	}()

	return nil
}

var (
	ErrCannotAwaitValue = errors.New("cannot await for value")

	ErrClosedChannel            = errors.New("closed channel")
	ErrCannotAssertTypeForValue = errors.New("cannot assert type for value")
)

func AwaitValue[T any](ec EventualConfig, key Key) (T, error) {
	ch, err := ec.GetValue(key)
	if err != nil {
		return *new(T), flaterrors.Join(err, ErrCannotAwaitValue)
	}

	v, ok := <-ch
	if !ok {
		return *new(T), flaterrors.Join(ErrClosedChannel, ErrCannotAwaitValue)
	}

	out, ok := v.(T)
	if !ok {
		return *new(T), flaterrors.Join(ErrCannotAssertTypeForValue, ErrCannotAwaitValue)
	}

	return out, nil
}
