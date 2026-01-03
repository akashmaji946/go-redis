package main

import (
	"fmt"
	"strconv"
)

const (
	STRING_TYPE = "string"
	HASH_TYPE   = "hash"
	LIST_TYPE   = "list"
	SET_TYPE    = "set"
	ZSET_TYPE   = "zset"
)

// ParseInt safely parses a string to int64
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ParseFloat safely parses a string to float64
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// NewErrorValue creates an error RESP value
func NewErrorValue(msg string) *Value {
	return &Value{
		typ: ERROR,
		err: msg,
	}
}

// NewStringValue creates a string RESP value
func NewStringValue(s string) *Value {
	return &Value{
		typ: STRING,
		str: s,
	}
}

// NewBulkValue creates a bulk string RESP value
func NewBulkValue(s string) *Value {
	return &Value{
		typ: BULK,
		blk: s,
	}
}

// NewIntegerValue creates an integer RESP value
func NewIntegerValue(n int64) *Value {
	return &Value{
		typ: INTEGER,
		num: int(n),
	}
}

// NewNullValue creates a null RESP value
func NewNullValue() *Value {
	return &Value{
		typ: NULL,
	}
}

// NewArrayValue creates an array RESP value
func NewArrayValue(arr []Value) *Value {
	return &Value{
		typ: ARRAY,
		arr: arr,
	}
}

// NewError creates an error for internal use
func NewError(msg string) error {
	return fmt.Errorf("%s", msg)
}

// Type checking helpers
func (item *Item) IsString() bool {
	return item.Type == STRING_TYPE || item.Type == "" // default to string
}

func (item *Item) IsHash() bool {
	return item.Type == HASH_TYPE
}

func (item *Item) IsList() bool {
	return item.Type == LIST_TYPE
}

func (item *Item) IsSet() bool {
	return item.Type == SET_TYPE
}

func (item *Item) IsZSet() bool {
	return item.Type == ZSET_TYPE
}

// Type enforcement helpers
func (item *Item) EnsureHash() error {
	if item.Type != "" && item.Type != HASH_TYPE {
		return NewError("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	if item.Hash == nil {
		item.Hash = make(map[string]string)
	}
	item.Type = HASH_TYPE
	return nil
}

func (item *Item) EnsureString() error {
	if item.Type != "" && item.Type != STRING_TYPE {
		return NewError("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	item.Type = STRING_TYPE
	return nil
}

// Factory functions
func NewStringItem(value string) *Item {
	return &Item{
		Str:  value,
		Type: STRING_TYPE,
	}
}

func NewHashItem() *Item {
	return &Item{
		Type: HASH_TYPE,
		Hash: make(map[string]string),
	}
}
