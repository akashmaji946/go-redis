package main

import (
	"bufio"
	"fmt"
	"io"
)

type Writer struct {
	writer io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		writer: bufio.NewWriter(w),
	}
}

func (w *Writer) Deserialize(v *Value) (reply string) {

	switch v.typ {
	case ARRAY:
		reply = fmt.Sprintf("%s%d%s", v.typ, len(v.arr), EOD)
		for _, val := range v.arr {
			reply += w.Deserialize(&val)
		}
	case STRING:
		reply = fmt.Sprintf("%s%s%s", v.typ, v.str, EOD) //+TATABYE\r\n
	case BULK:
		reply = fmt.Sprintf("%s%d%s%s%s", v.typ, len(v.blk), EOD, v.blk, EOD) //$5\r\nAkash\r\n
	case ERROR:
		reply = fmt.Sprintf("%s%s%s", v.typ, v.err, EOD) //-ERR msg\r\n
	case NULL:
		reply = fmt.Sprintf("%s%d%s", "$", -1, EOD) //$-1\r\n
	case INTEGER:
		reply = fmt.Sprintf("%s%d%s", v.typ, v.num, EOD) //:12\r\n
	default:
		fmt.Errorf("invalid typ given to Deserialize")
		return reply
	}

	return reply
}

func (w *Writer) Write(v *Value) {
	reply := w.Deserialize(v)
	// write and flush reply
	w.writer.Write([]byte(reply))
	// w.Flush() // flush now depends on sync state
}

func (w *Writer) Flush() {
	w.writer.(*bufio.Writer).Flush()
}
