package smc

import (
	"sort"
	"strings"
	"unicode"
)

func Camel(text string) string {
	text = strings.Map(func(chr rune) rune {
		if unicode.In(chr, unicode.Letter, unicode.Digit) {
			return chr
		}
		return 32
	}, text)
	text = strings.Title(text)
	text = strings.Map(func(chr rune) rune {
		if unicode.In(chr, unicode.Letter, unicode.Digit) {
			return chr
		}
		return -1
	}, text)
	return text
}

func SplitName(text string) (string, []string) {
	var tokens = strings.Split(text, ".")
	var last = len(tokens) - 1
	return tokens[last], tokens[:last]
}

func StringSet(list []string) []string {
	var dict = make(map[string]bool)
	for _, str := range list {
		dict[str] = true
	}
	list = nil
	for str, _ := range dict {
		list = append(list, str)
	}
	sort.Strings(list)
	return list
}
