package smc

import (
	"fmt"
	"io"
	"strings"
)

func PrintGo(file io.Writer, root *State, source []string) {
	var events = make(map[string]map[string][]*Event)
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\r\n")
	}
	var transition = func(idt int, event *Event) {
		var actions, dst = MakeTransition(event)
		if dst != nil && len(actions) != 0 {
			line(idt, "this.currentState = \"none\"")
		}
		for _, act := range actions {
			line(idt, "this.On%s()", Camel(act))
		}
		if dst != nil {
			line(idt, "this.currentState = \"%s\"", Camel(dst.Name()))
		}
	}
	var empty = func(events []*Event) bool {
		for _, event := range events {
			var actions, dst = MakeTransition(event)
			if dst != nil || len(actions) != 0 {
				return false
			}
		}
		return true
	}
	var name, ns = SplitName(root.Name())
	var allcond = root.AllConditions()
	var allact = root.AllActions()
	var allev = root.AllEvents()
	for _, state := range root.AllDescendants(root) {
		if state.IsLeaf() {
			events[state.Name()] = state.EventsGrouped()
		}
	}
	line(0, "package %s", strings.Join(ns, ""))
	line(0, "")
	line(0, "/**")
	line(0, strings.Join(source, "\r\n"))
	line(0, "**/")
	line(0, "")
	line(0, "type %s struct {", name)
	for _, act := range allact {
		line(1, "On%s func()", Camel(act))
	}
	for _, cond := range allcond {
		line(1, "Cond%s func() bool", Camel(cond))
	}
	line(1, "currentState string")
	line(0, "}")
	line(0, "")
	for _, evname := range allev {
		line(0, "func (this *%s) Send%s() {", name, Camel(evname))
		line(1, "switch this.currentState {")
		for _, state := range root.AllDescendants(root) {
			if state.IsNested() {
				continue
			}
			if evs, found := events[state.Name()][evname]; found {
				if empty(evs) {
					continue
				}
				line(1, "case \"%s\":", Camel(state.Name()))
				for _, event := range evs {
					if event.HasCond() {
						line(2, "if this.Cond%s() {", Camel(event.Cond()))
						transition(3, event)
						line(3, "return;")
						line(2, "}")
					} else {
						transition(2, event)
					}
				}
			}
		}
		line(1, "case \"none\":")
		line(2, "panic(\"invalid state\")")
		line(1, "}")
		line(0, "}")
	}
	line(0, "")
	var actions, dst = MakeStart(root)
	line(0, "func (this *%s) Start() {", name)
	line(1, "if this.currentState == \"\" {")
	for _, act := range actions {
		line(2, "this.On%s();", Camel(act))
	}
	line(2, "this.currentState = \"%s\"", Camel(dst.Name()))
	line(1, "}")
	line(0, "}")
}
