package smc

import (
	"fmt"
	"strings"
)

type Event struct {
	name string
	cond string
	src  *State
	dst  *State
	act  []string
}

func (event *Event) Name() string {
	return event.name
}

func (event *Event) Cond() string {
	return event.cond
}

func (event *Event) Src() *State {
	return event.src
}

func (event *Event) Dst() *State {
	return event.dst
}

func (event *Event) Actions() []string {
	return event.act
}

func (event *Event) IsInternal() bool {
	return event.dst == nil || event.src.IsDescendantOf(event.dst)
}

func (event *Event) HasCond() bool {
	return event.cond != ""
}

func (event *Event) Same(other *Event) bool {
	return event.Name() == other.Name() && event.Cond() == other.Cond()
}

func PrintEvent(event *Event, indent string) (lines []string) {
	var line string
	if event.cond == "" {
		line = fmt.Sprintf("%sevent %s", indent, event.name)
	} else {
		line = fmt.Sprintf("%sevent %s if %s", indent, event.name, event.cond)
	}
	if event.dst != nil || event.act != nil {
		line += " {"
		if event.dst != nil {
			line += fmt.Sprintf(" dst %s;", event.dst.name)
		}
		if event.act != nil {
			line += fmt.Sprintf(" act %s;", strings.Join(event.act, ", "))
		}
		line += " }"
	} else {
		line += ";"
	}
	lines = append(lines, line)
	return
}
