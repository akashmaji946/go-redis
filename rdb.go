package main

import (
	"encoding/gob"
	"fmt"
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

func InitRDBTrackers(conf *Config) {
	for _, rdb := range conf.rdb {
		tracker := NewSnapshotTracker(&rdb)
		trackers = append(trackers, tracker)

		go func() {
			defer tracker.ticker.Stop()

			for range tracker.ticker.C {
				fmt.Printf("keys changed = %d, needed = %d\n", tracker.keys, tracker.rdb.KeysChanged)
				if tracker.keys >= tracker.rdb.KeysChanged {
					SaveRDB(conf)
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

func SaveRDB(conf *Config) {
	fp := path.Join(conf.dir, conf.rdbFn) // file path
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening rdb file for saving", err)
		return
	}
	defer f.Close()

	// we will use gob
	err = gob.NewEncoder(f).Encode(&DB.store)
	if err != nil {
		fmt.Println("Error saving rdb file using gob: ", err)
		return
	}
	log.Println("saved rdb file")

}

func SyncRDB(conf *Config) {
	fp := path.Join(conf.dir, conf.rdbFn)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("Error opening rdb file for sync", err)
		return
	}
	defer f.Close()

	err = gob.NewDecoder(f).Decode(&DB.store)
	if err != nil {
		fmt.Println("Error restoring rdb file using gob: ", err)
		return
	}
	log.Println("restored rdb file")
}
