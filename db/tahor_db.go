package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	tahorCycleBucket    = "tahor_cycle"
	tahorPaymentsBucket = "tahor_payments"
	tahorLedgerBucket   = "tahor_ledger"
	tahorDelegateBucket = "tahor_delegate"
)

// TahorCycle holds the state of the current cleaning cycle
type TahorCycle struct {
	ID            string    `json:"id"` // e.g. "2026-Q1"
	StartedAt     time.Time `json:"started_at"`
	Active        bool      `json:"active"`
	FundCollected bool      `json:"fund_collected"`
	CleanerActive bool      `json:"cleaner_active"`
	UsedUnits     []string  `json:"used_units"` // units already selected as delegate
}

// TahorDelegate holds the current delegate info
type TahorDelegate struct {
	Unit     string    `json:"unit"`
	Account  string    `json:"account"`
	CycleID  string    `json:"cycle_id"`
	Selected time.Time `json:"selected"`
	Declined bool      `json:"declined"`
}

// TahorPayment holds a unit's cleaning fund payment
type TahorPayment struct {
	Unit      string    `json:"unit"`
	Amount    float64   `json:"amount"`
	CycleID   string    `json:"cycle_id"`
	Timestamp time.Time `json:"timestamp"`
}

// TahorLedger holds a debit entry (cleaner paid or materials bought)
type TahorLedger struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "cleaner" or "materials"
	Amount    float64   `json:"amount"`
	CycleID   string    `json:"cycle_id"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DB) InitTahorBuckets() error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{tahorCycleBucket, tahorPaymentsBucket, tahorLedgerBucket, tahorDelegateBucket, tahorCleaningBucket} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveTahorCycle saves the current cycle
func (d *DB) SaveTahorCycle(cycle *TahorCycle) error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCycleBucket))
		data, err := json.Marshal(cycle)
		if err != nil {
			return err
		}
		return b.Put([]byte("current"), data)
	})
}

// GetTahorCycle returns the current cycle
func (d *DB) GetTahorCycle() (*TahorCycle, error) {
	var cycle TahorCycle
	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCycleBucket))
		data := b.Get([]byte("current"))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &cycle)
	})
	if err != nil {
		return nil, err
	}
	if cycle.ID == "" {
		return nil, nil
	}
	return &cycle, nil
}

// SaveTahorDelegate saves the current delegate
func (d *DB) SaveTahorDelegate(delegate *TahorDelegate) error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorDelegateBucket))
		data, err := json.Marshal(delegate)
		if err != nil {
			return err
		}
		return b.Put([]byte("current"), data)
	})
}

// GetTahorDelegate returns the current delegate
func (d *DB) GetTahorDelegate() (*TahorDelegate, error) {
	var delegate TahorDelegate
	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorDelegateBucket))
		data := b.Get([]byte("current"))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &delegate)
	})
	if err != nil {
		return nil, err
	}
	if delegate.Unit == "" {
		return nil, nil
	}
	return &delegate, nil
}

// SaveTahorPayment saves a unit's fund payment
func (d *DB) SaveTahorPayment(cycleID, unit string, amount float64) (*TahorPayment, error) {
	key := cycleID + ":" + unit
	payment := &TahorPayment{
		Unit:      unit,
		Amount:    amount,
		CycleID:   cycleID,
		Timestamp: time.Now(),
	}
	err := d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorPaymentsBucket))
		data, err := json.Marshal(payment)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
	return payment, err
}

// GetTahorPayments returns all payments for a cycle
func (d *DB) GetTahorPayments(cycleID string) (map[string]*TahorPayment, error) {
	payments := make(map[string]*TahorPayment)
	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorPaymentsBucket))
		prefix := []byte(cycleID + ":")
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var p TahorPayment
			if err := json.Unmarshal(v, &p); err == nil {
				payments[p.Unit] = &p
			}
		}
		return nil
	})
	return payments, err
}

// AddTahorLedgerEntry adds a debit entry
func (d *DB) AddTahorLedgerEntry(cycleID, entryType string, amount float64) error {
	entry := &TahorLedger{
		ID:        entryType + ":" + time.Now().Format("2006-01-02T15:04:05"),
		Type:      entryType,
		Amount:    amount,
		CycleID:   cycleID,
		Timestamp: time.Now(),
	}
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorLedgerBucket))
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b.Put([]byte(entry.ID), data)
	})
}

// GetTahorLedger returns all ledger entries for a cycle
func (d *DB) GetTahorLedger(cycleID string) ([]*TahorLedger, error) {
	var entries []*TahorLedger
	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorLedgerBucket))
		return b.ForEach(func(k, v []byte) error {
			var entry TahorLedger
			if err := json.Unmarshal(v, &entry); err == nil && entry.CycleID == cycleID {
				entries = append(entries, &entry)
			}
			return nil
		})
	})
	return entries, err
}

// DeleteTahorPayment removes a unit's payment for the given cycle
func (d *DB) DeleteTahorPayment(cycleID, unit string) error {
	key := cycleID + ":" + unit
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorPaymentsBucket))
		return b.Delete([]byte(key))
	})
}

// CleaningSession holds a single confirmed cleaning session
type CleaningSession struct {
	Session   int       `json:"session"`
	CycleID   string    `json:"cycle_id"`
	Timestamp time.Time `json:"timestamp"`
}

const tahorCleaningBucket = "tahor_cleaning"

func (d *DB) InitCleaningBucket() error {
	return d.bolt.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(tahorCleaningBucket))
		return err
	})
}

// SaveCleaningSession saves a confirmed cleaning session
func (d *DB) SaveCleaningSession(cycleID string, session int) error {
	key := fmt.Sprintf("%s:%d", cycleID, session)
	entry := &CleaningSession{
		Session:   session,
		CycleID:   cycleID,
		Timestamp: time.Now(),
	}
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCleaningBucket))
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

// DeleteCleaningSession removes a cleaning session
func (d *DB) DeleteCleaningSession(cycleID string, session int) error {
	key := fmt.Sprintf("%s:%d", cycleID, session)
	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCleaningBucket))
		return b.Delete([]byte(key))
	})
}

// GetCleaningSessions returns all confirmed sessions for a cycle
func (d *DB) GetCleaningSessions(cycleID string) ([]int, error) {
	var sessions []int
	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCleaningBucket))
		prefix := []byte(cycleID + ":")
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var entry CleaningSession
			if err := json.Unmarshal(v, &entry); err == nil {
				sessions = append(sessions, entry.Session)
			}
		}
		return nil
	})
	return sessions, err
}

// IsSessionConfirmed checks if a session is already confirmed
func (d *DB) IsSessionConfirmed(cycleID string, session int) bool {
	key := fmt.Sprintf("%s:%d", cycleID, session)
	var found bool
	d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tahorCleaningBucket))
		found = b.Get([]byte(key)) != nil
		return nil
	})
	return found
}
