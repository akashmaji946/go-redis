package main

import (
	"io"
	"log"
	"strconv"
)

type ValueType string

const (
	CARRIAGE_RETURN string = "\r"
	NEW_LINE        string = "\n"
	EOD             string = "\r\n"
)

const (
	BULK   ValueType = "$"
	STRING ValueType = "+"
	ARRAY  ValueType = "*"

	ERROR ValueType = "-"
	NULL  ValueType = ""
)

type Value struct {
	typ ValueType

	blk string
	str string
	arr []Value

	err string
	nul string
}

func (v *Value) ReadBulk(reader io.Reader) Value {
	buf := make([]byte, 4)
	_, err := reader.Read(buf)
	if err != nil {
		log.Fatal("Can't read bulk")
	}

	n, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		log.Fatal("Can't convert to get bulk length")
	}

	bulkData := make([]byte, n+2) // data + \r\n
	reader.Read(bulkData)
	bulk := string(bulkData[:n]) // without \r\n

	return Value{
		typ: BULK,
		blk: bulk,
	}

}

func (v *Value) ReadArray(reader io.Reader) error {
	buf := make([]byte, 4)
	_, err := reader.Read(buf)
	if err != nil {
		log.Println("Can't read array.", err)
		return err
	}

	arrLen, err := strconv.Atoi(string(buf[1]))
	if err != nil {
		log.Println("Can't convert to get array length")
		return err
	}

	for range arrLen {
		bulk := v.ReadBulk(reader)
		v.arr = append(v.arr, bulk)
	}
	return nil
}
