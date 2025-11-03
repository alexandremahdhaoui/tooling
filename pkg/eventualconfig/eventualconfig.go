package eventualconfig

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
)

// Key defines a type for keys used in the EventualConfig.
type Key string

// EventualConfig provides an interface for a configuration that may be populated asynchronously.
// It allows setting a value for a key and getting a channel that will eventually receive that value.
type EventualConfig interface {
	// GetValue returns a channel that will receive the value for the given key.
	// It returns an error if the key was not declared at initialization.
	GetValue(key Key) (<-chan any, error)

	// SetValue sets the value for a given key.
	// It returns an error if the key was not declared at initialization.
	SetValue(key Key, value any) error
}

// NewEventualConfig creates a new EventualConfig with the given keys.
// The keys must be declared at initialization to be used later.
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
	// ErrCannotGetValue is returned when a value cannot be retrieved.
	ErrCannotGetValue = errors.New("cannot get value")
	// ErrValueMustBeDeclared is returned when a key is used without being declared at initialization.
	ErrValueMustBeDeclared = errors.New("value must be declared at initialization")
)

// GetValue returns a channel that will receive the value for the given key.
// It is the implementation of the EventualConfig interface.
func (ec *eventualConfig) GetValue(key Key) (<-chan any, error) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if ec.m[key] == nil {
		return nil, flaterrors.Join(errWithKey(key), ErrValueMustBeDeclared, ErrCannotGetValue)
	}

	return ec.m[key], nil
}

// ErrCannotSetValue is returned when a value cannot be set.
var ErrCannotSetValue = errors.New("cannot set value")

// SetValue sets the value for a given key.
// It is the implementation of the EventualConfig interface.
func (ec *eventualConfig) SetValue(key Key, value any) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if ec.m[key] == nil {
		return flaterrors.Join(errWithKey(key), ErrValueMustBeDeclared, ErrCannotSetValue)
	}

	go func() {
		for {
			ec.m[key] <- value
		}
	}()

	return nil
}

var (
	// ErrCannotAwaitValue is returned when a value cannot be awaited.
	ErrCannotAwaitValue = errors.New("cannot await for value")

	// ErrClosedChannel is returned when trying to read from a closed channel.
	ErrClosedChannel = errors.New("closed channel")
	// ErrCannotAssertTypeForValue is returned when a value cannot be asserted to the expected type.
	ErrCannotAssertTypeForValue = errors.New("cannot assert type for value")
)

// AwaitValue waits for a value to be set for a given key and returns it.
// It blocks until the value is available.
// It returns an error if the value cannot be awaited or if the type assertion fails.
func AwaitValue[T any](ec EventualConfig, key Key) (T, error) { //nolint:ireturn
	ch, err := ec.GetValue(key)
	if err != nil {
		return *new(T), flaterrors.Join(err, ErrCannotAwaitValue)
	}

	v, ok := <-ch
	if !ok {
		return *new(T), flaterrors.Join(errWithKey(key), ErrClosedChannel, ErrCannotAwaitValue)
	}

	out, ok := v.(T)
	if !ok {
		return *new(T), flaterrors.Join(
			errWithKeyAndValueOfType(key, v),
			ErrCannotAssertTypeForValue,
			ErrCannotAwaitValue)
	}

	return out, nil
}

func errWithKey(key Key) error {
	return fmt.Errorf("with key: %q", key) //nolint:err113
}

func errWithKeyAndValueOfType(key Key, v any) error {
	return flaterrors.Join(
		errWithKey(key),
		fmt.Errorf("with value: %#v", v),                      //nolint:err113
		fmt.Errorf("of type: %q", reflect.TypeOf(v).String()), //nolint:err113
	)
}
