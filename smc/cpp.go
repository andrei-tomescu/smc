package smc

import (
	"fmt"
	"io"
	"strings"
)

func PrintCpp(file io.Writer, root *State, source []string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\r\n")
	}
	var transition = func(idt int, event *Event) {
		var actions, dst = MakeTransition(event)
		if dst != nil && len(actions) != 0 {
			line(idt, "parent->SetInvalidState();")
		}
		for _, act := range actions {
			line(idt, "parent->On%s();", Camel(act))
		}
		if dst != nil {
			line(idt, "parent->SetState%s();", Camel(dst.Name()))
		}
	}
	var name, ns = SplitName(root.Name())
	var allcond = root.AllConditions()
	var allact = root.AllActions()
	var allev = root.AllEvents()
	line(0, "#pragma once")
	line(0, "")
	line(0, "/**")
	line(0, strings.Join(source, "\r\n"))
	line(0, "**/")
	line(0, "")
	line(0, "namespace %s {", strings.Join(ns, "::"))
	line(1, "struct %s {", name)
	for _, ev := range allev {
		line(2, "void Send%s() {", Camel(ev))
		line(3, "CurrentState->On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range allev {
		line(2, "void Post%s() {", Camel(ev))
		line(3, "PostEvent(&%s::Send%s);", name, Camel(ev))
		line(2, "}")
	}
	line(2, "void Start() {")
	line(3, "if (CurrentState == nullptr) {")
	var actions, dst = MakeStart(root)
	for _, act := range actions {
		line(4, "On%s();", Camel(act))
	}
	line(4, "SetState%s();", Camel(dst.Name()))
	line(3, "}")
	line(2, "}")
	line(2, "using Event = void (%s::*)();", name)
	line(1, "protected:")
	for _, act := range allact {
		line(2, "virtual void On%s() {", Camel(act))
		line(2, "}")
	}
	for _, cond := range allcond {
		line(2, "virtual bool Cond%s() const {", Camel(cond))
		line(3, "throw \"not implemented: Cond%s\";", Camel(cond))
		line(2, "}")
	}
	line(2, "virtual void PostEvent(Event event) {")
	line(3, "throw \"not implemented: PostEvent\";")
	line(2, "}")
	line(2, "void ProcessEvent(Event event) {")
	line(3, "(this->*event)();")
	line(2, "}")
	line(1, "private:")
	line(2, "struct IState {")
	for _, ev := range allev {
		line(3, "virtual void On%s(%s *) {", Camel(ev), name)
		line(3, "}")
	}
	line(2, "};")
	line(2, "struct InvalidState: IState {")
	for _, ev := range allev {
		line(3, "void On%s(%s *) override {", Camel(ev), name)
		line(4, "throw \"invalid state\";")
		line(3, "}")
	}
	line(2, "};")
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		line(2, "struct State%s: IState {", Camel(state.Name()))
		var groups = state.EventsGrouped()
		for _, evname := range allev {
			if events, found := groups[evname]; found {
				line(3, "void On%s(%s *parent) override {", Camel(evname), name)
				for _, event := range events {
					if event.HasCond() {
						line(4, "if (parent->Cond%s()) {", Camel(event.Cond()))
						transition(5, event)
						line(5, "return;")
						line(4, "}")
					} else {
						transition(4, event)
					}
				}
				line(3, "}")
			}
		}
		line(2, "};")
	}
	line(2, "void SetInvalidState() {")
	line(3, "static InvalidState Instance;")
	line(3, "CurrentState = &Instance;")
	line(2, "}")
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		line(2, "void SetState%s() {", Camel(state.Name()))
		line(3, "static State%s Instance;", Camel(state.Name()))
		line(3, "CurrentState = &Instance;")
		line(2, "}")
	}
	line(2, "IState *CurrentState = nullptr;")
	line(1, "};")
	line(0, "}")
}
