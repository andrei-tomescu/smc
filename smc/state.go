package smc

import (
	"fmt"
	"strings"
)

type State struct {
	name   string
	start  *State
	parent *State
	entry  []string
	exit   []string
	nested []*State
	events []*Event
}

func (state *State) Name() string {
	return state.name
}

func (state *State) Start() *State {
	return state.start
}

func (state *State) FollowStart() *State {
	if state.IsLeaf() {
		return state
	}
	if state.start == nil {
		panic(state.Name() + ": missing start")
	}
	if state.start.IsDescendantOf(state) {
		return state.start.FollowStart()
	}
	panic(state.Name() + ": invalid start")
}
func (state *State) Parent() *State {
	return state.parent
}

func (state *State) Entry() []string {
	return state.entry
}

func (state *State) Exit() []string {
	return state.exit
}

func (state *State) Children() []*State {
	return state.nested
}

func (state *State) AllDescendants(states ...*State) []*State {
	for _, child := range state.nested {
		states = append(states, child)
		states = child.AllDescendants(states...)
	}
	return states
}

func (state *State) IsNested() bool {
	return len(state.nested) != 0
}

func (state *State) IsLeaf() bool {
	return len(state.nested) == 0
}

func (state *State) IsDescendantOf(other *State) bool {
	if state == other {
		return true
	}
	if state.parent == nil {
		return false
	}
	return state.parent.IsDescendantOf(other)
}

func (state *State) Path() []*State {
	if state.parent == nil {
		return []*State{state}
	}
	return append(state.parent.Path(), state)
}

func (state *State) Diff(other *State) ([]*State, []*State) {
	var src, dst = state.Path(), other.Path()
	for len(src) != 0 && len(dst) != 0 && src[0] == dst[0] {
		src = src[1:]
		dst = dst[1:]
	}
	return src, dst
}

func (state *State) AddState(other *State) {
	state.nested = append(state.nested, other)
}

func (state *State) Events() []*Event {
	return state.events
}

func (state *State) EventsGrouped() map[string][]*Event {
	var groups = make(map[string][]*Event)
	for _, event := range state.events {
		if event.HasCond() {
			groups[event.Name()] = append(groups[event.Name()], event)
		}
	}
	for _, event := range state.events {
		if event.HasCond() == false {
			groups[event.Name()] = append(groups[event.Name()], event)
		}
	}
	return groups
}

func (state *State) AddEvent(event *Event) bool {
	for _, ev := range state.events {
		if event.Same(ev) {
			return true
		}
	}
	state.events = append(state.events, event)
	return false
}

func (state *State) PushEvents() {
	for _, child := range state.Children() {
		child.PushEvents()
	}
	for _, child := range state.AllDescendants() {
		for _, event := range state.Events() {
			child.AddEvent(&Event{
				event.name,
				event.cond,
				child,
				event.dst,
				event.act,
			})
		}
	}
}

func (root *State) AllEvents() []string {
	var all []string
	for _, state := range root.AllDescendants(root) {
		for _, event := range state.Events() {
			all = append(all, event.Name())
		}
	}
	return StringSet(all)
}

func (root *State) AllConditions() []string {
	var all []string
	for _, state := range root.AllDescendants(root) {
		for _, event := range state.Events() {
			if event.HasCond() {
				all = append(all, event.Cond())
			}
		}
	}
	return StringSet(all)
}

func (root *State) AllActions() []string {
	var all []string
	for _, state := range root.AllDescendants(root) {
		for _, event := range state.Events() {
			all = append(all, event.Actions()...)
		}
		all = append(all, state.Entry()...)
		all = append(all, state.Exit()...)
	}
	return StringSet(all)
}

func PrintState(state *State, indent string) (lines []string) {
	var line = func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	if state.start != nil || state.entry != nil || state.exit != nil || state.nested != nil || state.events != nil {
		if state.name == "" {
			line("%sstate {", indent)
		} else {
			line("%sstate %s {", indent, state.name)
		}
		if state.entry != nil {
			line("%s\tentry %s;", indent, strings.Join(state.entry, ", "))
		}
		if state.exit != nil {
			line("%s\texit %s;", indent, strings.Join(state.exit, ", "))
		}
		if state.start != nil {
			line("%s\tstart %s;", indent, state.start.name)
		}
		for _, st := range state.nested {
			lines = append(lines, PrintState(st, indent+"\t")...)
		}
		for _, ev := range state.events {
			lines = append(lines, PrintEvent(ev, indent+"\t")...)
		}
		line("%s}", indent)
		return
	}
	line("%sstate %s;", indent, state.name)
	return
}

func PrintRoot(state *State, indent string) (lines []string) {
	var line = func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}
	line("%s%s {", indent, state.name)
	if state.entry != nil {
		line("%s\tentry %s;", indent, strings.Join(state.entry, ", "))
	}
	if state.exit != nil {
		line("%s\texit %s;", indent, strings.Join(state.exit, ", "))
	}
	if state.start != nil {
		line("%s\tstart %s;", indent, state.start.name)
	}
	for _, st := range state.nested {
		lines = append(lines, PrintState(st, indent+"\t")...)
	}
	for _, ev := range state.events {
		lines = append(lines, PrintEvent(ev, indent+"\t")...)
	}
	line("%s}", indent)
	return
}
