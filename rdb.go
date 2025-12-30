package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

type SnapshotTracker struct {
	keys   int
	ticker time.Ticker
	rdb    *RDBSnapshot
}

func NewSnapshotTracker(rdb *RDBSnapshot) *SnapshotTracker {
	return &SnapshotTracker{
		keys:   0,
		ticker: *time.NewTicker(time.Second * time.Duration(rdb.Secs)),
		rdb:    rdb,
	}
}

var trackers = []*SnapshotTracker{}

func InitRDBTrackers(conf *Config, state *AppState) {
	for _, rdb := range conf.rdb {
		tracker := NewSnapshotTracker(&rdb)
		trackers = append(trackers, tracker)

		go func() {
			defer tracker.ticker.Stop()

			for range tracker.ticker.C {
				// fmt.Printf("keys changed = %d, needed = %d\n", tracker.keys, tracker.rdb.KeysChanged)
				if tracker.keys >= tracker.rdb.KeysChanged {
					SaveRDB(state)
				}
				tracker.keys = 0
			}

		}()

	}
}

func IncrRDBTrackers() {
	for _, t := range trackers {
		t.keys += 1
	}
}

func SaveRDB(state *AppState) {
	fp := path.Join(state.config.dir, state.config.rdbFn) // file path
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("error opening rdb file for saving", err)
		return
	}
	defer f.Close()

	log.Println("saving rdb file")

	log.Println("copying data from DB to buffer....")
	var buf bytes.Buffer
	// if background saving is on, save the copy
	if state.bgsaving {
		err = gob.NewEncoder(&buf).Encode(&state.DBCopy)
	} else {
		// we will use gob
		DB.mu.RLock()
		err = gob.NewEncoder(&buf).Encode(&DB.store) // not thread safe so use lock
		DB.mu.RUnlock()
	}

	if err != nil {
		fmt.Println("error copying DB to buf", err)
		return
	}

	data := buf.Bytes() //actual DB data

	// compute checksums and compare
	// checksum of buffer data
	bsum, err := Hash(&buf)
	if err != nil {
		log.Println("can't compute buf checksum bsum: ", err)
		return
	}
	fmt.Println("copying data from from buf to file")

	_, err = f.Write(data)
	if err != nil {
		fmt.Printf("error copying data from from buf to file")
		return
	}
	if err := f.Sync(); err != nil {
		log.Println("can't sync files and flush to disk: ", err)
		return
	}

	// checksum of file data
	f.Seek(0, io.SeekStart)
	fsum, err := Hash(f)
	if err != nil {
		log.Println("can't compute file checksum fsum:", err)
		return
	}
	// fmt.Printf("checksums:\nfsum=%s\nbsum=%s\n", fsum, bsum)
	// compare
	if bsum != fsum {
		fmt.Printf("checksum mismatch:\nfsum=%s\nbsum=%s\n", fsum, bsum)
		return
	}

	log.Println("saved rdb file")

}

func SyncRDB(conf *Config, state *AppState) {
	fp := path.Join(conf.dir, conf.rdbFn)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("error opening rdb file for sync", err)
		f.Close()
		return
	}
	defer f.Close()

	err = gob.NewDecoder(f).Decode(&DB.store)
	if err != nil {
		fmt.Println("error restoring rdb file using gob: ", err)
		return
	}
	log.Println("restored rdb file")
}

func Hash(r io.Reader) (string, error) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	if err != nil {
		log.Println("can't copy from reader to hash")
		return "", err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("HASH = %s\n", hash)
	return hash, nil
}
