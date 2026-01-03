package main

import (
	"fmt"
	"strconv"
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
