package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
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

	INTEGER ValueType = ":"
)

type Value struct {
	typ ValueType

	blk string
	str string
	arr []Value

	err string
	nul string

	num int
}

func ReadLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n') // line = *123\r\n
	if err != nil {
		return "", err
	}
	trimmedLine := strings.TrimSuffix(line, "\r\n") // line = *123
	return trimmedLine, nil
}

func (v *Value) ReadBulk(reader *bufio.Reader) Value {

	line, err := ReadLine(reader) // line = $1123
	if err != nil {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}
	if line[0] != '$' {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}

	n, err := strconv.Atoi(string(line[1:])) // "1123"
	if err != nil {
		log.Fatal("can't convert to get bulk length")
	}

	bulkDataBuffer := make([]byte, n+2) // data + \r\n

	// read till filling the buffer
	_, err = io.ReadFull(reader, bulkDataBuffer)
	if err != nil {
		fmt.Println("error in ReadBulk:", err)
		return Value{
			typ: NULL,
		}
	}
	bulkData := string(bulkDataBuffer[:n]) // data without \r\n

	return Value{
		typ: BULK,
		blk: bulkData,
	}

}

func (v *Value) ReadArray(r io.Reader) error {

	reader := bufio.NewReader(r)
	line, err := ReadLine(reader) // line = *123
	if err != nil {
		fmt.Println("error in ReadArray:", err)
		return err
	}
	if line[0] != '*' {
		fmt.Println("error in ReadArray:", err)
		return fmt.Errorf("invalid input by user")
	}

	arrLen, err := strconv.Atoi(line[1:]) // pass "123"
	if err != nil {
		log.Println("can't convert to get array length")
		return err
	}

	for range arrLen {
		bulk := v.ReadBulk(reader)
		v.arr = append(v.arr, bulk)
	}
	return nil
}
