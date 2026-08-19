package bbolt

import (
	"encoding/binary"

	bolt "go.etcd.io/bbolt"
)

const (
	bucketMeta         = "meta"
	bucketEvents       = "events"
	bucketStreams      = "streams"
	bucketIDIndex      = "id_index"
	bucketCommandIndex = "command_index"

	counterHead       = "head"
	counterEventCount = "event_count"
)

// encodeUint64BE encodes a uint64 as 8 big-endian bytes.
// This is the native sort order for bbolt's B-tree.
func encodeUint64BE(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// decodeUint64BE decodes 8 big-endian bytes to uint64.
func decodeUint64BE(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// streamKey returns the key for the stream version counter.
// Key: "<tenant>\x00<streamID>" → value = uint64BE(currentVersion).
func streamKey(tenant, streamID string) []byte {
	return []byte(tenant + "\x00" + streamID)
}

// streamVersionKey returns the key for a specific version of a stream.
// Key: "<tenant>\x00<streamID>\x00<version>" → value = uint64BE(position).
func streamVersionKey(tenant, streamID string, version uint64) []byte {
	return append([]byte(tenant+"\x00"+streamID+"\x00"), encodeUint64BE(version)...)
}

// decodeStreamVersionFromKey extracts the version from a streamVersionKey.
func decodeStreamVersionFromKey(key []byte) uint64 {
	// Key format: "<tenant>\x00<streamID>\x00<version>"
	// Version is the last 8 bytes.
	return binary.BigEndian.Uint64(key[len(key)-8:])
}

// hasPrefix checks if key starts with prefix.
func hasPrefix(key, prefix []byte) bool {
	if len(key) < len(prefix) {
		return false
	}
	for i := range prefix {
		if key[i] != prefix[i] {
			return false
		}
	}
	return true
}

// readHead reads the global head counter from the meta bucket.
func readHead(tx *bolt.Tx) (uint64, error) {
	meta := tx.Bucket([]byte(bucketMeta))
	if meta == nil {
		return 0, nil
	}
	v := meta.Get([]byte(counterHead))
	if v == nil {
		return 0, nil
	}
	return decodeUint64BE(v), nil
}

// writeHead writes the global head counter to the meta bucket.
func writeHead(tx *bolt.Tx, head uint64) error {
	meta := tx.Bucket([]byte(bucketMeta))
	if meta == nil {
		return nil
	}
	return meta.Put([]byte(counterHead), encodeUint64BE(head))
}

// incrementCounter increments a named counter in the meta bucket.
func incrementCounter(tx *bolt.Tx, name string) error {
	meta := tx.Bucket([]byte(bucketMeta))
	if meta == nil {
		return nil
	}
	v := meta.Get([]byte(name))
	var count uint64
	if v != nil {
		count = decodeUint64BE(v)
	}
	count++
	return meta.Put([]byte(name), encodeUint64BE(count))
}
