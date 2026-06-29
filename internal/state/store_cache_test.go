package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Cache(t *testing.T) {
	store, err := NewStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	t.Run("SetAndGetCache", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		err := store.SetCache(key, value, 0)
		require.NoError(t, err)

		retrieved, err := store.GetCache(key)
		require.NoError(t, err)
		assert.Equal(t, value, retrieved)
	})

	t.Run("GetCache_NotFound", func(t *testing.T) {
		_, err := store.GetCache("non-existent-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache key not found")
	})

	t.Run("SetCache_Update", func(t *testing.T) {
		key := "update-key"
		value1 := "value1"
		value2 := "value2"

		err := store.SetCache(key, value1, 0)
		require.NoError(t, err)

		err = store.SetCache(key, value2, 0)
		require.NoError(t, err)

		retrieved, err := store.GetCache(key)
		require.NoError(t, err)
		assert.Equal(t, value2, retrieved)
	})

	t.Run("SetCache_WithTTL", func(t *testing.T) {
		key := "ttl-key"
		value := "ttl-value"
		ttl := 100 * time.Millisecond

		err := store.SetCache(key, value, ttl)
		require.NoError(t, err)

		// Should be available immediately
		retrieved, err := store.GetCache(key)
		require.NoError(t, err)
		assert.Equal(t, value, retrieved)

		// Wait for expiration
		time.Sleep(200 * time.Millisecond)

		// Should be gone
		_, err = store.GetCache(key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache key not found")
	})

	t.Run("CheckCacheTableExists", func(t *testing.T) {
		// Verify that the table was created by migrations
		var count int
		err := store.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='cache'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "cache table should exist")
	})
}
