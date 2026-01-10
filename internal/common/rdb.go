/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/rdb.go
*/
package common

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
	for _, rdb := range conf.Rdb {
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
	fp := path.Join(state.Config.Dir, state.Config.RdbFn) // file path
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
	if state.Bgsaving {
		err = gob.NewEncoder(&buf).Encode(&state.DBCopy)
	} else {
		// we will use gob
		// DB.Mu.RLock()
		// err = gob.NewEncoder(&buf).Encode(&DB.store) // not thread safe so use lock
		// DB.Mu.RUnlock()

		// For now, just skip saving if not bgsaving
		err = fmt.Errorf("RDB save requires DBCopy to be set")
		return
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

	// save stats
	state.RdbStats.RDBLastSavedTS = time.Now().Unix()
	state.RdbStats.RDBSavesCount += 1
	log.Println("saved rdb file")

}

func SyncRDB(conf *Config, state *AppState) {
	fp := path.Join(conf.Dir, conf.RdbFn)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("error opening rdb file for sync", err)
		f.Close()
		return
	}
	defer f.Close()

	// err = gob.NewDecoder(f).Decode(&DB.store)
	// if err != nil {
	// 	log.Println("error restoring rdb file using gob: ", err)
	// 	return
	// }

	// Recompute in-memory accounting after restoring DB.store from RDB.
	// For now, skip memory accounting
	// var total int64 = 0
	// for k, item := range DB.store {
	// 	if item == nil {
	// 		continue
	// 	}
	// 	total += item.ApproxMemoryUsage(k)
	// }
	// DB.Mu.Lock()
	// DB.mem = total
	// if DB.mem > DB.mempeak {
	// 	DB.mempeak = DB.mem
	// }
	// DB.Mu.Unlock()

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
