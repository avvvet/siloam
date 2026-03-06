package db

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	readingsBucket = "readings"
	billsBucket    = "bills"
	paymentsBucket = "payments"
)

type Reading struct {
	Unit      string    `json:"unit"`
	Value     int       `json:"value"`
	OldValue  int       `json:"old_value,omitempty"`
	Updated   bool      `json:"updated"`
	Timestamp time.Time `json:"timestamp"`
	Month     string    `json:"month"`
}

type Bill struct {
	Month     string             `json:"month"`
	TotalBill float64            `json:"total_bill"`
	Units     map[string]float64 `json:"units"`     // unit -> amount owed
	Diffs     map[string]int     `json:"diffs"`     // unit -> usage difference
	Percents  map[string]float64 `json:"percents"`  // unit -> percentage
	Previous  map[string]int     `json:"previous"`  // unit -> previous reading
	Current   map[string]int     `json:"current"`   // unit -> current reading
	Finalized bool               `json:"finalized"` // true after midnight calculation
	UpdatedAt time.Time          `json:"updated_at"`
}

type Payment struct {
	Unit      string    `json:"unit"`
	Amount    float64   `json:"amount"`
	OldAmount float64   `json:"old_amount,omitempty"`
	Updated   bool      `json:"updated"`
	Timestamp time.Time `json:"timestamp"`
	Month     string    `json:"month"`
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
		for _, bucket := range []string{readingsBucket, billsBucket, paymentsBucket} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &DB{bolt: boltDB}, nil
}

func (d *DB) Close() error {
	return d.bolt.Close()
}

// monthKey returns current month key e.g. "2026-03"
func monthKey() string {
	return time.Now().Format("2006-01")
}

// unitKey returns composite key: "2026-03:a"
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

// SaveBill saves or updates the bill for the current month
func (d *DB) SaveBill(bill *Bill) error {
	month := monthKey()
	bill.Month = month
	bill.UpdatedAt = time.Now()

	return d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(billsBucket))
		data, err := json.Marshal(bill)
		if err != nil {
			return err
		}
		return b.Put([]byte(month), data)
	})
}

// GetBill returns the bill for the current month
func (d *DB) GetBill() (*Bill, error) {
	month := monthKey()
	var bill Bill

	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(billsBucket))
		data := b.Get([]byte(month))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &bill)
	})

	if err != nil {
		return nil, err
	}
	if bill.Month == "" {
		return nil, nil
	}
	return &bill, nil
}

// GetPreviousReadings returns the previous month's readings as a map unit -> value
func (d *DB) GetPreviousReadings() (map[string]int, error) {
	now := time.Now()
	prevMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
	readings := make(map[string]int)

	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(readingsBucket))
		prefix := []byte(prevMonth + ":")
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var r Reading
			if err := json.Unmarshal(v, &r); err == nil {
				readings[r.Unit] = r.Value
			}
		}
		return nil
	})

	return readings, err
}

// SavePayment saves or overwrites a unit payment for the current month
func (d *DB) SavePayment(unit string, amount float64) (*Payment, error) {
	month := monthKey()
	key := unitKey(month, unit)
	var payment Payment

	err := d.bolt.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(paymentsBucket))
		existing := b.Get([]byte(key))
		if existing != nil {
			var prev Payment
			if err := json.Unmarshal(existing, &prev); err == nil {
				payment.OldAmount = prev.Amount
				payment.Updated = true
			}
		}
		payment.Unit = unit
		payment.Amount = amount
		payment.Month = month
		payment.Timestamp = time.Now()
		data, err := json.Marshal(payment)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})

	return &payment, err
}

// GetAllPayments returns all payments for the current month
func (d *DB) GetAllPayments() (map[string]*Payment, error) {
	month := monthKey()
	payments := make(map[string]*Payment)

	err := d.bolt.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(paymentsBucket))
		prefix := []byte(month + ":")
		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = c.Next() {
			var p Payment
			if err := json.Unmarshal(v, &p); err == nil {
				payments[p.Unit] = &p
			}
		}
		return nil
	})

	return payments, err
}
