package plugin

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	Mt "github.com/maroda/monteverdi/types"
)

type BadgerOutput struct {
	MU        sync.Mutex
	DB        *badger.DB
	BatchSize int
	Buffer    []*Mt.PulseEvent
}

func NewBadgerOutput(path string, batchSize int) (*BadgerOutput, error) {
	opts := badger.DefaultOptions(path).
		WithCompression(options.ZSTD).
		WithNumVersionsToKeep(1)

	db, err := badger.Open(opts)
	if err != nil {
		slog.Error("BadgerOutput failed to open database", slog.Any("error", err))
		return nil, fmt.Errorf("database error: %w", err)
	}

	slog.Info("BadgerOutput opened",
		slog.String("path", path),
		slog.Int("batchSize", batchSize))

	return &BadgerOutput{
		DB:        db,
		BatchSize: batchSize,
		Buffer:    make([]*Mt.PulseEvent, 0, batchSize),
	}, nil
}

// WritePulse queues up a batch of pulses,
// when batchsize is reached, it calls Flush()
// which calls WriteBatch() with the new batch
func (bo *BadgerOutput) WritePulse(pulse *Mt.PulseEvent) error {
	bo.MU.Lock()
	defer bo.MU.Unlock()

	bo.Buffer = append(bo.Buffer, pulse)
	if len(bo.Buffer) >= bo.BatchSize {
		return bo.flushLocked() // private Flush that does not lock
	}
	return nil
}

// WriteBatch performs the key/value creation to be stored
// and actually calls BadgerDB to write the data
func (bo *BadgerOutput) WriteBatch(pulses []*Mt.PulseEvent) error {
	wb := bo.DB.NewWriteBatch()
	defer wb.Cancel()

	for _, p := range pulses {
		k := PulseKey(p)
		v := PulseEncode(p)
		if err := wb.Set(k, v); err != nil {
			slog.Error("BadgerOutput failed to set key in batch",
				slog.Any("error", err),
				slog.Time("pulseTime", p.StartTime),
				slog.String("metric", p.Metric[0]))
			return fmt.Errorf("write batch error: %w", err)
		}
	}

	if err := wb.Flush(); err != nil {
		slog.Error("BadgerOutput failed to flush batch", slog.Any("error", err))
		return fmt.Errorf("batch flush error: %w", err)
	}

	return nil
}

// Flush is the public method that blocks,
// it sends data to WriteBatch and then clears the buffer
func (bo *BadgerOutput) Flush() error {
	bo.MU.Lock()
	defer bo.MU.Unlock()

	if len(bo.Buffer) == 0 {
		return nil
	}

	err := bo.WriteBatch(bo.Buffer) // Delegate to WriteBatch
	bo.Buffer = bo.Buffer[:0]       // Clear but keep capacity
	return err
}

// flushLocked mimics Flush without locking, called by WritePulse
func (bo *BadgerOutput) flushLocked() error {
	err := bo.WriteBatch(bo.Buffer) // Delegate to WriteBatch
	bo.Buffer = bo.Buffer[:0]       // Clear but keep capacity
	return err
}

// Close returns a Flush error but still attempts to close
func (bo *BadgerOutput) Close() error {
	slog.Info("BadgerOutput closing, flushing buffer",
		slog.Int("bufferSize", len(bo.Buffer)))
	flushErr := bo.Flush()
	closeErr := bo.DB.Close()

	if flushErr != nil {
		slog.Error("BadgerOutput failed to flush on close", slog.Any("error", flushErr))
		return fmt.Errorf("flush failed, close may have failed: %v", flushErr)
	}

	if closeErr != nil {
		slog.Error("BadgerOutput failed to close database", slog.Any("error", closeErr))
		return fmt.Errorf("close failed: %v", closeErr)
	}

	slog.Info("BadgerOutput closed successfully")
	return nil
}

func (bo *BadgerOutput) Type() string { return "BadgerDB" }

// PulseKey creates a composite key
// timestamp + dimension + first five letters of metric
func PulseKey(pulse *Mt.PulseEvent) []byte {
	key := make([]byte, 8+1+5)

	// Using positive BigEndian integer to convert timestamp
	// so keys can be sorted chronologically by BadgerDB
	binary.BigEndian.PutUint64(key[0:8], uint64(pulse.StartTime.UnixNano()))

	// Set Dimension
	key[8] = byte(pulse.Dimension)

	// Keep metric name at five chars
	if len(pulse.Metric) > 0 {
		mBytes := []byte(pulse.Metric[0])
		n := len(mBytes)
		if n > 5 {
			n = 5
		}
		copy(key[9:9+n], mBytes[:n])
	}

	return key
}

// PulseEncode serializes the pulse event struct for data storage
func PulseEncode(p *Mt.PulseEvent) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(p)
	return buf.Bytes()
}

// PulseDecode deserializes the pulse event data
func PulseDecode(data []byte) (*Mt.PulseEvent, error) {
	var p Mt.PulseEvent
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&p)
	return &p, err
}

// QueryRange retrieves pulses within a time range
func (bo *BadgerOutput) QueryRange(start, end time.Time) ([]*Mt.PulseEvent, error) {
	var pulses []*Mt.PulseEvent

	// db.View() callback
	// BadgerDB provides a transaction in which to get item.Value()
	err := bo.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			// item.Value() callback
			// BadgerDB passes bytes to the anon func
			err := item.Value(func(val []byte) error {
				pulse, err := PulseDecode(val)
				if err != nil {
					slog.Error("BadgerOutput failed to decode pulse", slog.Any("error", err))
					return fmt.Errorf("pulse decode error: %w", err)
				}

				// Filter by time range
				if pulse.StartTime.After(start) && pulse.StartTime.Before(end) {
					pulses = append(pulses, pulse)
				}

				return nil
			})
			if err != nil {
				slog.Error("BadgerOutput callback failure", slog.Any("error", err))
				return fmt.Errorf("item data error: %w", err)
			}
		}
		return nil
	})

	slog.Info("BadgerOutput QueryRange successful", slog.Int("count", len(pulses)))

	return pulses, err
}
