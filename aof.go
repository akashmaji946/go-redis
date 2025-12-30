package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

type Aof struct {
	w      *Writer
	f      *os.File
	config *Config
}

func NewAof(config *Config) *Aof {
	aof := Aof{
		config: config,
	}
	fp := path.Join(aof.config.dir, aof.config.aofFn)                  //filepath
	f, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644) // append only + readwrite // perm = -rw-r--r--

	if err != nil {
		fmt.Println("Can't open this file path")
		return &aof
	}

	aof.w = NewWriter(f)
	aof.f = f

	return &aof
}

func (aof *Aof) Synchronize() {

	total := 0
	for {
		v := Value{}
		err := v.ReadArray(aof.f)
		if err == io.EOF {
			break
		}
		if err != nil { // can't sync
			fmt.Println("Unexpected error while sync", err)
			break
		}

		config := Config{}                 //empty
		blankState := NewAppState(&config) // new blank state with empty config
		blankClient := Client{}            // dummy

		Set(&blankClient, &v, blankState)

		total += 1
		// fmt.Println(v)
	}
	log.Printf("records synchronized: %d\n", total)

}

func (aof *Aof) Rewrite(cp map[string]*VAL) {
	// future SET commands will go to to buffer
	var b bytes.Buffer
	aof.w = NewWriter(&b) // writer to buffer

	// we have copy of DB in cp, so remoev file
	err := aof.f.Truncate(0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Truncate")
		return
	}
	_, err = aof.f.Seek(0, 0)
	if err != nil {
		log.Println("ERR AOF Rewrite issue! Can't Seek")
		return
	}

	// write all k, v as SET k, v into truncated file(no duplicates!)
	fwriter := NewWriter(aof.f) // writer to file
	for k, v := range cp {
		cmd := Value{typ: BULK, blk: "SET"}
		key := Value{typ: BULK, blk: k}     // string
		value := Value{typ: BULK, blk: v.v} // actual string

		arr := Value{typ: ARRAY, arr: []Value{cmd, key, value}}
		fwriter.Write(&arr)
	}
	fwriter.Flush()
	log.Println("done BGREWRITE.")

	// if buffer b is not empty, write it as well
	if _, err := b.WriteTo(aof.f); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't append buffered commands:", err)
		return
	} else if err := aof.f.Sync(); err != nil {
		log.Println("ERR AOF Rewrite issue! Can't sync after appending buffer:", err)
		return
	}

	// if b.Len() > 0 {
	// 	// Flush the writer to ensure all buffered data is in the bytes.Buffer
	// 	aof.w.Flush()

	// 	// Append the buffered commands to the file
	// 	_, err = aof.f.Write(b.Bytes())
	// 	if err != nil {
	// 		log.Println("ERR AOF Rewrite issue! Can't append buffered commands:", err)
	// 		return
	// 	}

	// 	// Sync to ensure data is written to disk
	// 	if err := aof.f.Sync(); err != nil {
	// 		log.Println("ERR AOF Rewrite issue! Can't sync after appending buffer:", err)
	// 		return
	// 	}
	// }

	// rewrite to file
	aof.w = NewWriter(aof.f)

}
