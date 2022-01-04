package smc

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func Main() {
	defer func() {
		if msg := recover(); msg != nil {
			fmt.Println(msg)
			os.Exit(1)
		}
	}()
	if len(os.Args) < 3 {
		panic("usage: smc [cs|cpp|go] <file>")
	}
	var (
		root = Scan(strings.NewReader(ReadRoot(os.Args[2])))
		src  = PrintRoot(root, "")
		buf  = bytes.NewBuffer(nil)
	)
	root.PushEvents()
	if os.Args[1] == "cs" {
		PrintCs(buf, root, src)
		CheckWriteFile(os.Args[2], buf.Bytes())
	}
	if os.Args[1] == "cpp" {
		PrintCpp(buf, root, src)
		CheckWriteFile(os.Args[2], buf.Bytes())
	}
	if os.Args[1] == "go" {
		PrintGo(buf, root, src)
		CheckWriteFile(os.Args[2], buf.Bytes())
	}
	if os.Args[1] == "lms-cs" {
		PrintLmsCs(buf, root, src)
		CheckWriteFile(os.Args[2], buf.Bytes())
	}
}
