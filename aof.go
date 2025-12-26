package main

import (
	"fmt"
	"io"
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
		Set(&v, blankState)
		fmt.Println(v)
	}

}
