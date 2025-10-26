package eventualconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventualConfig(t *testing.T) {
	t.Run("NewEventualConfig", func(t *testing.T) {
		t.Parallel()

		t.Run("should create a new EventualConfig with the given keys", func(t *testing.T) {
			t.Parallel()

			keys := []Key{"key1", "key2"}
			ec := NewEventualConfig(keys...)

			require.NotNil(t, ec)

			for _, key := range keys {
				_, err := ec.GetValue(key)
				assert.NoError(t, err)
			}
		})
	})

	t.Run("GetValue", func(t *testing.T) {
		t.Parallel()

		t.Run("should return a channel for a declared key", func(t *testing.T) {
			t.Parallel()

			key := Key("declaredKey")
			ec := NewEventualConfig(key)

			ch, err := ec.GetValue(key)
			assert.NoError(t, err)
			assert.NotNil(t, ch)
		})

		t.Run("should return an error for an undeclared key", func(t *testing.T) {
			t.Parallel()

			ec := NewEventualConfig()
			key := Key("undeclaredKey")

			ch, err := ec.GetValue(key)
			assert.Error(t, err)
			assert.Nil(t, ch)
		})
	})

	t.Run("SetValue", func(t *testing.T) {
		t.Parallel()

		t.Run("should set a value for a declared key", func(t *testing.T) {
			t.Parallel()

			key := Key("declaredKey")
			value := "testValue"
			ec := NewEventualConfig(key)

			err := ec.SetValue(key, value)
			assert.NoError(t, err)

			ch, _ := ec.GetValue(key)
			receivedValue := <-ch
			assert.Equal(t, value, receivedValue)
		})

		t.Run("should return an error for an undeclared key", func(t *testing.T) {
			t.Parallel()

			ec := NewEventualConfig()
			key := Key("undeclaredKey")
			value := "testValue"

			err := ec.SetValue(key, value)
			assert.Error(t, err)
		})

		t.Run("should support setting value concurrently", func(t *testing.T) {
			t.Parallel()

			key := Key("concurrentKey")
			value := "concurrentValue"
			ec := NewEventualConfig(key)

			go func() {
				err := ec.SetValue(key, value)
				assert.NoError(t, err)
			}()

			ch, err := ec.GetValue(key)
			require.NoError(t, err)

			select {
			case receivedValue := <-ch:
				assert.Equal(t, value, receivedValue)
			case <-time.After(1 * time.Second):
				t.Fatal("timed out waiting for value")
			}
		})
	})

	t.Run("AwaitValue", func(t *testing.T) {
		t.Parallel()

		t.Run("should await and return a value of the correct type", func(t *testing.T) {
			t.Parallel()

			key := Key("awaitKey")
			value := "awaitValue"
			ec := NewEventualConfig(key)

			go func() {
				time.Sleep(10 * time.Millisecond) // Ensure AwaitValue blocks
				err := ec.SetValue(key, value)
				assert.NoError(t, err)
			}()

			retrievedValue, err := AwaitValue[string](ec, key)
			assert.NoError(t, err)
			assert.Equal(t, value, retrievedValue)
		})

		t.Run("should return an error for an undeclared key", func(t *testing.T) {
			t.Parallel()

			ec := NewEventualConfig()
			key := Key("undeclaredAwaitKey")

			_, err := AwaitValue[string](ec, key)
			assert.Error(t, err)
		})

		t.Run("should return an error if the value is of the wrong type", func(t *testing.T) {
			t.Parallel()

			key := Key("wrongTypeKey")
			value := 123 // int, not string
			ec := NewEventualConfig(key)

			go func() {
				err := ec.SetValue(key, value)
				assert.NoError(t, err)
			}()

			_, err := AwaitValue[string](ec, key)
			assert.Error(t, err)
		})

		t.Run("should return an error if the channel is closed", func(t *testing.T) {
			t.Parallel()

			key := Key("closedChanKey")
			ec := NewEventualConfig(key)

			_, err := ec.GetValue(key)
			require.NoError(t, err)

			// Close the channel directly for testing purposes.
			// In a real-world scenario, the channel owner would handle this.
			close(ec.(*eventualConfig).m[key])

			_, err = AwaitValue[string](ec, key)
			assert.Error(t, err)
		})
	})
}
