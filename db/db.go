package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	readingsBucket = "readings"
)

type Reading struct {
	Unit      string    `json:"unit"`
	Value     int       `json:"value"`
	OldValue  int       `json:"old_value,omitempty"`
	Updated   bool      `json:"updated"`
	Timestamp time.Time `json:"timestamp"`
	Month     string    `json:"month"` // format: "2006-01"
}

type DB struct {
	bolt *bolt.DB
}

func Open(path string) (*DB, error) {
	boltDB, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	err = boltDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(readingsBucket))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &DB{bolt: boltDB}, nil
}

func (d *DB) Close() error {
	return d.bolt.Close()
}

// monthKey returns a bucket key for the current month e.g. "2026-03"
func monthKey() string {
	return time.Now().Format("2006-01")
}

// unitKey returns a composite key: "2026-03:a"
func unitKey(month, unit string) string {
	return fmt.Sprintf("%s:%s", month, unit)
}

// SaveReading saves or overwrites a unit reading for the current month
func (d *DB) SaveReading(unit string, value int) (*Reading, error) {
	month := monthKey()
	key := unitKey(month, unit)

	var reading Reading

	err := d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(readingsBucket))

		// Check if exists
		existing := b.Get([]byte(key))
		if existing != nil {
			var prev Reading
			if err := json.Unmarshal(existing, &prev); err == nil {
				reading.OldValue = prev.Value
				reading.Updated = true
			}
		}

		reading.Unit = unit
		reading.Value = value
		reading.Month = month
		reading.Timestamp = time.Now()

		data, err := json.Marshal(reading)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})

	return &reading, err
}

// GetAllReadings returns all readings for the current month
func (d *DB) GetAllReadings() (map[string]*Reading, error) {
	month := monthKey()
	readings := make(map[string]*Reading)

	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(readingsBucket))
		prefix := []byte(month + ":")

		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r Reading
			if err := json.Unmarshal(v, &r); err == nil {
				readings[r.Unit] = &r
			}
		}
		return nil
	})

	return readings, err
}
