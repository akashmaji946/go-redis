/*
author: akashmaji
email: akashmaji@iisc.ac.in
file: go-redis/rdb.go
*/
package common

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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
	dbID   int
}

func NewSnapshotTracker(rdb *RDBSnapshot, dbID int) *SnapshotTracker {
	return &SnapshotTracker{
		keys:   0,
		ticker: *time.NewTicker(time.Second * time.Duration(rdb.Secs)),
		rdb:    rdb,
		dbID:   dbID,
	}
}

func (tr *SnapshotTracker) Incr() {
	tr.keys++
}

func InitRDBTrackers(conf *Config, state *AppState, dbID int, getSnapshot func() map[string]*Item) []*SnapshotTracker {
	var dbTrackers []*SnapshotTracker
	for _, rdb := range conf.Rdb {
		tracker := NewSnapshotTracker(&rdb, dbID)
		dbTrackers = append(dbTrackers, tracker)

		go func(tr *SnapshotTracker) {
			defer tr.ticker.Stop()
			for range tr.ticker.C {
				if tr.keys >= tr.rdb.KeysChanged {
					log.Printf("[..RDB..] Automatic saving triggered for DB %d", tr.dbID)
					SaveRDB(state, tr.dbID, getSnapshot())
				}
				tr.keys = 0
			}
		}(tracker)
	}
	return dbTrackers
}

// SaveRDB writes a gob-encoded snapshot to disk. It expects that when
// state.Bgsaving is true a prepared copy exists in state.DBCopy. To avoid
// truncating the RDB file prematurely, the file is opened only after the
// buffer is prepared.
func SaveRDB(state *AppState, dbID int, data map[string]*Item) {
	log.Printf("saving rdb file for DB %d", dbID)

	log.Println("copying data from DB to buffer....")
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&data); err != nil {
		fmt.Println("error copying DB to buf", err)
		return
	}

	encodedData := buf.Bytes()

	if state.Config.Encrypt {
		key := sha256.Sum256([]byte(state.Config.Nonce))
		block, _ := aes.NewCipher(key[:])
		gcm, _ := cipher.NewGCM(block)
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			log.Println("failed to generate nonce:", err)
			return
		}
		encodedData = gcm.Seal(nonce, nonce, encodedData, nil)
	}

	// checksum of buffer data
	bsum, err := Hash(bytes.NewReader(encodedData))
	if err != nil {
		log.Println("can't compute buf checksum bsum: ", err)
		return
	}

	// Now open the file for writing/truncation and write data
	filename := fmt.Sprintf("%s%d.rdb", state.Config.RdbFn, dbID)
	fp := path.Join(state.Config.Dir, filename)
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("error opening rdb file for saving", err)
		return
	}
	defer f.Close()

	log.Println("saveRDB: copying data from from buf to file")
	if _, err := f.Write(encodedData); err != nil {
		fmt.Printf("error copying data from from buf to file")
		return
	}
	if err := f.Sync(); err != nil {
		log.Println("can't sync files and flush to disk: ", err)
		return
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Println("can't seek file to start:", err)
		return
	}
	fsum, err := Hash(f)
	if err != nil {
		log.Println("can't compute file checksum fsum:", err)
		return
	}
	if bsum != fsum {
		fmt.Printf("checksum mismatch:\nfsum=%s\nbsum=%s\n", fsum, bsum)
		return
	}

	state.RdbStats.RDBLastSavedTS = time.Now().Unix()
	state.RdbStats.RDBSavesCount += 1
	log.Println("saved rdb file")
}

// SyncRDB decodes the rdb file and returns the restored map for the caller
// to apply to the in-memory database (avoids import cycles).
func SyncRDB(conf *Config, state *AppState, dbID int) (map[string]*Item, error) {
	filename := fmt.Sprintf("%s%d.rdb", conf.RdbFn, dbID)
	fp := path.Join(conf.Dir, filename)
	f, err := os.OpenFile(fp, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		fmt.Println("error opening rdb file for sync", err)
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() == 0 {
		return nil, nil
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if conf.Encrypt {
		key := sha256.Sum256([]byte(conf.Nonce))
		block, _ := aes.NewCipher(key[:])
		gcm, _ := cipher.NewGCM(block)
		nonceSize := gcm.NonceSize()
		if len(content) < nonceSize {
			return nil, fmt.Errorf("ciphertext too short")
		}
		nonce, ciphertext := content[:nonceSize], content[nonceSize:]
		content, err = gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("decryption failed: %v", err)
		}
	}

	var restored map[string]*Item
	dec := gob.NewDecoder(bytes.NewReader(content))
	if err := dec.Decode(&restored); err != nil {
		log.Println("error restoring rdb file using gob: ", err)
		return nil, err
	}

	log.Println("restored rdb file")
	return restored, nil
}

func Hash(r io.Reader) (string, error) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	if err != nil {
		log.Println("can't copy from reader to hash")
		return "", err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	log.Printf("HASH = %s\n", hash)
	return hash, nil
}
