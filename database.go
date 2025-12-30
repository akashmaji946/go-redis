package main

import (
	"sync"
	"time"
)

// sample database
// var DB = map[string]string{}

// database has store(actual data) and a mutex(concurrency)
type VAL struct {
	v   string
	exp time.Time
}

type Database struct {
	store map[string]*VAL
	mu    sync.RWMutex
}

func NewDatabase() *Database {
	return &Database{
		store: map[string]*VAL{},
		mu:    sync.RWMutex{},
	}
}

func (db *Database) Put(k string, v string) {
	DB.store[k] = &VAL{v: v}
}

func (db *Database) Poll(k string) (val *VAL, ok bool) {
	Val, ok := DB.store[k]
	if ok != true {
		return &VAL{}, ok
	}
	return Val, ok
}

func (db *Database) Del(k string) {
	delete(DB.store, k)
}

var DB = NewDatabase()
