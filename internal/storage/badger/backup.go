package badger

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/pb"
	"google.golang.org/protobuf/proto"
)

const (
	bitDelete      = 1 << 0 // Badger's delete marker in KV.Meta
	maxBackupFrame = 256 << 20
)

// Backup writes the log lines of the days between from and to (YYYY-MM-DD,
// inclusive, both optional) in Badger's backup format. Read offsets, metadata
// and the index stay out: a restore rebuilds the index and must never move
// the read positions of the node it lands on.
func (s *Store) Backup(w io.Writer, from, to string) (int64, error) {
	var n atomic.Int64
	st := s.db.NewStream()
	st.LogPrefix = "kapture.Backup"
	st.ChooseKey = func(item *badgerdb.Item) bool {
		k := item.Key()
		if len(k) < 11 || k[0] < '0' || k[0] > '9' || item.IsDeletedOrExpired() {
			return false
		}
		if d := string(k[:10]); (from != "" && d < from) || (to != "" && d > to) {
			return false
		}
		n.Add(1)
		return true
	}
	_, err := st.Backup(w, 0)
	return n.Load(), err
}

// Restore loads a backup made by Backup, from any node or time zone. Lines get
// fresh versions, move to the day of their timestamp in this store's zone and
// follow this store's retention. Lines already stored are overwritten, so a
// restore can be repeated. The index is rebuilt at the end.
func (s *Store) Restore(r io.Reader) (int64, error) {
	n, err := s.load(r)
	if n > 0 {
		if ierr := s.reindex(); ierr != nil && err == nil {
			err = fmt.Errorf("rebuild index: %w", ierr)
		}
	}
	return n, err
}

func (s *Store) load(r io.Reader) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	br := bufio.NewReaderSize(r, 1<<16)
	wb := s.db.NewWriteBatch()
	defer wb.Cancel()
	now := time.Now()
	var n int64
	var buf []byte
	for {
		var size uint64
		if err := binary.Read(br, binary.LittleEndian, &size); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return n, flushAfter(wb, fmt.Errorf("read backup: %w", err))
		}
		if size > maxBackupFrame {
			return n, flushAfter(wb, errors.New("not a Kapture backup stream"))
		}
		if uint64(cap(buf)) < size {
			buf = make([]byte, size)
		}
		buf = buf[:size]
		if _, err := io.ReadFull(br, buf); err != nil {
			return n, flushAfter(wb, fmt.Errorf("read backup: %w", err))
		}
		var list pb.KVList
		if err := proto.Unmarshal(buf, &list); err != nil { // copies the bytes, buf is reused
			return n, flushAfter(wb, fmt.Errorf("not a Kapture backup stream: %w", err))
		}
		for _, kv := range list.Kv {
			if (len(kv.Meta) > 0 && kv.Meta[0]&bitDelete != 0) || len(kv.Value) == 0 {
				continue
			}
			k, ok := parseKey(kv.Key)
			if !ok {
				continue
			}
			e := badgerdb.NewEntry(nil, kv.Value)
			if s.opts.Retention > 0 {
				expires := time.Unix(0, k.ts).Add(s.opts.Retention)
				if expires.Before(now) {
					continue
				}
				e.ExpiresAt = uint64(expires.Unix())
			}
			k.date = dateOf(k.ts, s.opts.Location)
			e.Key = encodeKey(k)
			if err := wb.SetEntry(e); err != nil {
				return n, fmt.Errorf("restore: %w", err)
			}
			n++
		}
	}
	return n, wb.Flush()
}

// flushAfter keeps what was read before a damaged part of the stream.
func flushAfter(wb *badgerdb.WriteBatch, err error) error {
	if ferr := wb.Flush(); ferr != nil {
		return errors.Join(err, ferr)
	}
	return err
}

// reindex rebuilds the rollups and catalog from the stored keys. Writes wait
// meanwhile, so no line is missed between the scan and the swap.
func (s *Store) reindex() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rebuildIndex(); err != nil {
		return err
	}
	return s.idx.load(s.db)
}
