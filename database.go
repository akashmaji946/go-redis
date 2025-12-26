package main

import "sync"

// sample database
// var DB = map[string]string{}

// database has store(actual data) and a mutex(concurrency)
type Database struct {
	store map[string]string
	mu    sync.RWMutex
}

func NewDatabase() *Database {
	return &Database{
		store: map[string]string{},
		mu:    sync.RWMutex{},
	}
}

var DB = NewDatabase()
