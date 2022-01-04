package smc

import (
	"bytes"
	"os"
	"strings"
)

func CheckWriteFile(filename string, text []byte) {
	if data, err := os.ReadFile(filename); err == nil {
		if bytes.Equal(text, data) {
			return
		}
	}
	if err := os.WriteFile(filename, text, 0666); err != nil {
		panic("unable to create file " + filename)
	}
}

func ReadRoot(filename string) string {
	if data, err := os.ReadFile(filename); err == nil {
		var text = string(data)
		if first := strings.Index(text, "/**") + 3; first != 2 {
			if last := strings.Index(text, "**/"); last != -1 {
				return text[first:last]
			} else {
				panic(filename + ": expecting /** ... **/")
			}
		} else {
			panic(filename + ": expecting /** ... **/")
		}
	} else {
		panic(err)
	}
}
