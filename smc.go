package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/scanner"
	"unicode"
)

/******************************************************************************/

type State struct {
	name   string
	start  *State
	parent *State
	entry  []string
	exit   []string
	nested []*State
	events []*Event
}

type Event struct {
	name string
	cond string
	src  *State
	dst  *State
	act  []string
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

/******************************************************************************/

func MakeEntry(path []*State) ([]string, *State) {
	if len(path) == 0 {
		return nil, nil
	}
	if path[0].IsLeaf() {
		return path[0].Entry(), path[0]
	}
	var act, dst = MakeEntry(path[1:])
	return append(path[0].Entry(), act...), dst
}

func MakeExit(path []*State) []string {
	if len(path) == 0 {
		return nil
	}
	return append(MakeExit(path[1:]), path[0].Exit()...)
}

func MakeTransition(event *Event) ([]string, *State) {
	if event.IsInternal() {
		return event.Actions(), nil
	} else {
		var expath, enpath = event.Src().Diff(event.Dst().FollowStart())
		var exit = MakeExit(expath)
		var actions = event.Actions()
		var entry, dst = MakeEntry(enpath)
		return append(exit, append(actions, entry...)...), dst
	}
}

func MakeStart(root *State) ([]string, *State) {
	return MakeEntry(root.FollowStart().Path())
}

/******************************************************************************/

func AllEvents(root *State) []string {
	var all []string
	for _, state := range root.AllDescendants(root) {
		for _, event := range state.Events() {
			all = append(all, event.Name())
		}
	}
	return StringSet(all)
}

func AllConditions(root *State) []string {
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

func AllActions(root *State) []string {
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

/******************************************************************************/

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

/******************************************************************************/

func CheckWriteFile(filename string, text []byte) {
	if data, err := ioutil.ReadFile(filename); err == nil {
		if bytes.Equal(text, data) {
			return
		}
	}
	if err := ioutil.WriteFile(filename, text, 0666); err != nil {
		panic("unable to create file " + filename)
	}
}

/******************************************************************************/

func main() {
	defer func() {
		if msg := recover(); msg != nil {
			fmt.Println(msg)
			os.Exit(1)
		}
	}()
	if len(os.Args) != 4 {
		panic("usage: smc [cs|cpp|go] <input-file> <output-file>")
	}
	var file, err = os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	var root = Scan(file)
	var src = PrintRoot(root, "")
	root.PushEvents()
	if os.Args[1] == "cs" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCs(buffer, root, src)
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
	if os.Args[1] == "cpp" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCpp(buffer, root, src)
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
	if os.Args[1] == "go" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenGo(buffer, root, src)
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
}

/******************************************************************************/

func CodeGenCs(file io.Writer, root *State, source []string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\r\n")
	}
	var transition = func(idt int, event *Event) {
		var actions, dst = MakeTransition(event)
		if dst != nil && len(actions) != 0 {
			line(idt, "parent.CurrentState = InvalidState.Instance;")
		}
		for _, act := range actions {
			line(idt, "parent.Handler.On%s();", Camel(act))
		}
		if dst != nil {
			line(idt, "parent.CurrentState = State%s.Instance;", Camel(dst.Name()))
		}
	}
	var name, ns = SplitName(root.Name())
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "using System;")
	line(0, "")
	line(0, "namespace %s {", strings.Join(ns, "."))
	line(1, "public sealed class %s {", name)
	line(2, "public interface IHandler {")
	for _, cond := range allcond {
		line(3, "bool Cond%s();", Camel(cond))
	}
	for _, act := range allact {
		line(3, "void On%s();", Camel(act))
	}
	line(3, "void PostEvent(Action action);")
	line(2, "}")
	line(2, "public sealed class DelegateHandler: IHandler {")
	for _, cond := range allcond {
		line(3, "public bool Cond%s() {", Camel(cond))
		line(4, "return cond%s();", Camel(cond))
		line(3, "}")
		line(3, "public Func<bool> cond%s { get; set; }", Camel(cond))
	}
	for _, act := range allact {
		line(3, "public void On%s() {", Camel(act))
		line(4, "on%s();", Camel(act))
		line(3, "}")
		line(3, "public Action on%s { get; set; }", Camel(act))
	}
	line(3, "public void PostEvent(Action action) {")
	line(4, "postEvent(action);")
	line(3, "}")
	line(3, "public Action<Action> postEvent { get; set; }")
	line(2, "}")
	for _, ev := range allev {
		line(2, "public void Send%s() {", Camel(ev))
		line(3, "CurrentState.On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range allev {
		line(2, "public void Post%s() {", Camel(ev))
		line(3, "Handler.PostEvent(Send%s);", Camel(ev))
		line(2, "}")
	}
	line(2, "public void Start() {")
	line(3, "if (CurrentState == null) {")
	var actions, dst = MakeStart(root)
	for _, act := range actions {
		line(4, "Handler.On%s();", Camel(act))
	}
	line(4, "CurrentState = State%s.Instance;", Camel(dst.Name()))
	line(3, "}")
	line(2, "}")
	line(2, "private class IState {")
	for _, ev := range AllEvents(root) {
		line(3, "public virtual void On%s(%s parent) {", Camel(ev), name)
		line(3, "}")
	}
	line(2, "}")
	line(2, "private class InvalidState: IState {")
	for _, ev := range AllEvents(root) {
		line(3, "public override void On%s(%s parent) {", Camel(ev), name)
		line(4, "throw new Exception();")
		line(3, "}")
	}
	line(3, "public static readonly IState Instance = new InvalidState();")
	line(2, "}")
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		line(2, "private class State%s: IState {", Camel(state.Name()))
		var groups = state.EventsGrouped()
		for _, evname := range allev {
			if events, found := groups[evname]; found {
				line(3, "public override void On%s(%s parent) {", Camel(evname), name)
				for _, event := range events {
					if event.HasCond() {
						line(4, "if (parent.Handler.Cond%s()) {", Camel(event.Cond()))
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
		line(3, "public static readonly IState Instance = new State%s();", Camel(state.Name()))
		line(2, "}")
	}
	line(2, "public %s(IHandler handler) {", name)
	line(3, "Handler = handler;")
	line(2, "}")
	line(2, "private readonly IHandler Handler;")
	line(2, "private IState CurrentState;")
	line(1, "}")
	line(0, "}")
	line(0, "")
	line(0, "/*")
	line(0, strings.Join(source, "\r\n"))
	line(0, "*/")
}

/******************************************************************************/

func CodeGenCpp(file io.Writer, root *State, source []string) {
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "#pragma once")
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
	line(0, "")
	line(0, "/*")
	line(0, strings.Join(source, "\r\n"))
	line(0, "*/")
}

/******************************************************************************/

func CodeGenGo(file io.Writer, root *State, source []string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\r\n")
	}
	var transition = func(idt int, event *Event) {
		var actions, dst = MakeTransition(event)
		if dst != nil && len(actions) != 0 {
			line(idt, "current = nil")
		}
		for _, act := range actions {
			line(idt, "this.On%s()", Camel(act))
		}
		if dst != nil {
			line(idt, "current = fn%s", Camel(dst.Name()))
		}
	}
	var name, ns = SplitName(root.Name())
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "package %s", strings.Join(ns, "_"))
	line(0, "")
	line(0, "type %s struct {", name)
	for _, ev := range allev {
		line(1, "Send%s func()", Camel(ev))
	}
	line(1, "Start func()")
	for _, act := range allact {
		line(1, "On%s func()", Camel(act))
	}
	for _, cond := range allcond {
		line(1, "Cond%s func() bool", Camel(cond))
	}
	line(0, "}")
	line(0, "")
	line(0, "func New%s() *%s {", name, name)
	line(1, "var (")
	for _, state := range root.AllDescendants(root) {
		if state.IsLeaf() {
			line(2, "fn%s []func()", Camel(state.Name()))
		}
	}
	line(2, "current []func()")
	line(1, ")")
	line(1, "var this = %s{", name)
	for idx, ev := range allev {
		line(2, "Send%s: func() {", Camel(ev))
		line(3, "current[%d]()", idx)
		line(2, "},")
	}
	line(2, "Start: func() {")
	line(3, "if current == nil {")
	var actions, dst = MakeStart(root)
	for _, act := range actions {
		line(4, "this.On%s()", Camel(act))
	}
	line(4, "current = fn%s", Camel(dst.Name()))
	line(3, "}")
	line(2, "},")
	line(1, "}")
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		line(1, "fn%s = []func() {", Camel(state.Name()))
		var groups = state.EventsGrouped()
		for _, evname := range allev {
			line(2, "func() {")
			if events, found := groups[evname]; found {
				for _, event := range events {
					if event.HasCond() {
						line(3, "if this.Cond%s() {", Camel(event.Cond()))
						transition(4, event)
						line(4, "return")
						line(3, "}")
					} else {
						transition(3, event)
					}
				}
			}
			line(2, "},")
		}
		line(1, "}")
	}
	line(1, "return &this")
	line(0, "}")
	line(0, "")
	line(0, "/*")
	line(0, strings.Join(source, "\r\n"))
	line(0, "*/")
}

/******************************************************************************/

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

/******************************************************************************/

func Scan(file io.Reader) *State {
	var (
		root   *State
		state  *State
		event  *Event
		scan   scanner.Scanner
		next   rune
		names  = make(map[string][]func(*State))
		parser = NewParser()
	)
	parser.OnErrorUnexpected = func() {
		panic(scan.Pos().String() + ": unexpected " + scan.TokenText())
	}
	parser.OnEventAct = func() {
		event.act = append(event.act, scan.TokenText())
	}
	parser.OnEventBegin = func() {
		event = &Event{src: state}
	}
	parser.OnEventCond = func() {
		event.cond = scan.TokenText()
	}
	parser.OnEventDst = func() {
		var this = event
		var text = scan.TokenText()
		names[text] = append(names[text], func(st *State) {
			this.dst = st
		})
	}
	parser.OnEventEnd = func() {
		if state.AddEvent(event) {
			panic(scan.Pos().String() + ": event " + event.Name() + " redeclared")
		}
	}
	parser.OnEventName = func() {
		event.name = scan.TokenText()
	}
	parser.OnRootBegin = func() {
		root = &State{name: scan.TokenText()}
		state = root
	}
	parser.OnRootName = func() {
		root.name += "." + scan.TokenText()
	}
	parser.OnStateBegin = func() {
		if state == nil {
			parser.OnErrorUnexpected()
		}
		state = &State{parent: state}
	}
	parser.OnStateEnd = func() {
		if state == nil {
			parser.OnErrorUnexpected()
		}
		if state.parent != nil {
			state.parent.AddState(state)
		}
		state = state.parent
	}
	parser.OnStateEntry = func() {
		state.entry = append(state.entry, scan.TokenText())
	}
	parser.OnStateExit = func() {
		state.exit = append(state.exit, scan.TokenText())
	}
	parser.OnStateName = func() {
		state.name = scan.TokenText()
	}
	parser.OnStateStart = func() {
		var this = state
		var text = scan.TokenText()
		names[text] = append(names[text], func(st *State) {
			this.start = st
		})
	}
	parser.CondAct = func() bool {
		return scan.TokenText() == "act"
	}
	parser.CondBra = func() bool {
		return scan.TokenText() == "{"
	}
	parser.CondComma = func() bool {
		return scan.TokenText() == ","
	}
	parser.CondDot = func() bool {
		return scan.TokenText() == "."
	}
	parser.CondDst = func() bool {
		return scan.TokenText() == "dst"
	}
	parser.CondEntry = func() bool {
		return scan.TokenText() == "entry"
	}
	parser.CondEvent = func() bool {
		return scan.TokenText() == "event"
	}
	parser.CondExit = func() bool {
		return scan.TokenText() == "exit"
	}
	parser.CondIdent = func() bool {
		return next == scanner.Ident
	}
	parser.CondIf = func() bool {
		return scan.TokenText() == "if"
	}
	parser.CondKet = func() bool {
		return scan.TokenText() == "}"
	}
	parser.CondSemi = func() bool {
		return scan.TokenText() == ";"
	}
	parser.CondStart = func() bool {
		return scan.TokenText() == "start"
	}
	parser.CondState = func() bool {
		return scan.TokenText() == "state"
	}
	parser.Start()
	scan.Init(file)
	scan.Mode = scanner.ScanIdents | scanner.ScanComments | scanner.SkipComments
	for next = scan.Scan(); next != scanner.EOF; next = scan.Scan() {
		parser.SendNext()
	}
	if state != nil || root == nil {
		panic(scan.Pos().String() + ": unexpected EOF")
	}
	for _, st := range root.AllDescendants(root) {
		if list, ok := names[st.name]; ok {
			for _, fn := range list {
				fn(st)
			}
			names[st.name] = nil
		}
	}
	for name, list := range names {
		if len(list) != 0 {
			panic("unknown state " + name)
		}
	}
	return root
}

/******************************************************************************/

type Parser struct {
	SendNext          func()
	Start             func()
	OnErrorUnexpected func()
	OnEventAct        func()
	OnEventBegin      func()
	OnEventCond       func()
	OnEventDst        func()
	OnEventEnd        func()
	OnEventName       func()
	OnRootBegin       func()
	OnRootName        func()
	OnStateBegin      func()
	OnStateEnd        func()
	OnStateEntry      func()
	OnStateExit       func()
	OnStateName       func()
	OnStateStart      func()
	CondAct           func() bool
	CondBra           func() bool
	CondComma         func() bool
	CondDot           func() bool
	CondDst           func() bool
	CondEntry         func() bool
	CondEvent         func() bool
	CondExit          func() bool
	CondIdent         func() bool
	CondIf            func() bool
	CondKet           func() bool
	CondSemi          func() bool
	CondStart         func() bool
	CondState         func() bool
}

func NewParser() *Parser {
	var (
		fnRootBegin      []func()
		fnRootNext       []func()
		fnRootName       []func()
		fnStateEntry     []func()
		fnStateExit      []func()
		fnStateStart     []func()
		fnStateName      []func()
		fnStateNameNext  []func()
		fnStateStartNext []func()
		fnStateEntryNext []func()
		fnStateExitNext  []func()
		fnStateNext      []func()
		fnEventName      []func()
		fnEventCond      []func()
		fnEventAct       []func()
		fnEventDst       []func()
		fnEventNameNext  []func()
		fnEventCondNext  []func()
		fnEventDstNext   []func()
		fnEventActNext   []func()
		fnEventNext      []func()
		current          []func()
	)
	var this = Parser{
		SendNext: func() {
			current[0]()
		},
		Start: func() {
			if current == nil {
				current = fnRootBegin
			}
		},
	}
	fnRootBegin = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnRootBegin()
				current = fnRootNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnRootNext = []func(){
		func() {
			if this.CondBra() {
				current = fnStateNext
				return
			}
			if this.CondDot() {
				current = fnRootName
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnRootName = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnRootName()
				current = fnRootNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateEntry = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnStateEntry()
				current = fnStateEntryNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateExit = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnStateExit()
				current = fnStateExitNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateStart = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnStateStart()
				current = fnStateStartNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateName = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnStateName()
				current = fnStateNameNext
				return
			}
			if this.CondSemi() {
				current = nil
				this.OnStateEnd()
				current = fnStateNext
				return
			}
			if this.CondBra() {
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateNameNext = []func(){
		func() {
			if this.CondSemi() {
				current = nil
				this.OnStateEnd()
				current = fnStateNext
				return
			}
			if this.CondBra() {
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateStartNext = []func(){
		func() {
			if this.CondSemi() {
				current = fnStateNext
				return
			}
			if this.CondKet() {
				current = nil
				this.OnStateEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateEntryNext = []func(){
		func() {
			if this.CondComma() {
				current = fnStateEntry
				return
			}
			if this.CondSemi() {
				current = fnStateNext
				return
			}
			if this.CondKet() {
				current = nil
				this.OnStateEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateExitNext = []func(){
		func() {
			if this.CondComma() {
				current = fnStateExit
				return
			}
			if this.CondSemi() {
				current = fnStateNext
				return
			}
			if this.CondKet() {
				current = nil
				this.OnStateEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnStateNext = []func(){
		func() {
			if this.CondEntry() {
				current = fnStateEntry
				return
			}
			if this.CondEvent() {
				current = nil
				this.OnEventBegin()
				current = fnEventName
				return
			}
			if this.CondExit() {
				current = fnStateExit
				return
			}
			if this.CondStart() {
				current = fnStateStart
				return
			}
			if this.CondState() {
				current = nil
				this.OnStateBegin()
				current = fnStateName
				return
			}
			if this.CondSemi() {
				return
			}
			if this.CondKet() {
				this.OnStateEnd()
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventName = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnEventName()
				current = fnEventNameNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventCond = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnEventCond()
				current = fnEventCondNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventAct = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnEventAct()
				current = fnEventActNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventDst = []func(){
		func() {
			if this.CondIdent() {
				current = nil
				this.OnEventDst()
				current = fnEventDstNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventNameNext = []func(){
		func() {
			if this.CondIf() {
				current = fnEventCond
				return
			}
			if this.CondSemi() {
				current = nil
				this.OnEventEnd()
				current = fnStateNext
				return
			}
			if this.CondBra() {
				current = fnEventNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventCondNext = []func(){
		func() {
			if this.CondSemi() {
				current = nil
				this.OnEventEnd()
				current = fnStateNext
				return
			}
			if this.CondBra() {
				current = fnEventNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventDstNext = []func(){
		func() {
			if this.CondSemi() {
				current = fnEventNext
				return
			}
			if this.CondKet() {
				current = nil
				this.OnEventEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventActNext = []func(){
		func() {
			if this.CondComma() {
				current = fnEventAct
				return
			}
			if this.CondSemi() {
				current = fnEventNext
				return
			}
			if this.CondKet() {
				current = nil
				this.OnEventEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	fnEventNext = []func(){
		func() {
			if this.CondAct() {
				current = fnEventAct
				return
			}
			if this.CondDst() {
				current = fnEventDst
				return
			}
			if this.CondSemi() {
				return
			}
			if this.CondKet() {
				current = nil
				this.OnEventEnd()
				current = fnStateNext
				return
			}
			this.OnErrorUnexpected()
		},
	}
	return &this
}

/*
main.Parser {
	start root_begin;
	state root_begin {
		event next if ident { dst root_next; act root_begin; }
	}
	state root_next {
		event next if bra { dst state_next; }
		event next if dot { dst root_name; }
	}
	state root_name {
		event next if ident { dst root_next; act root_name; }
	}
	state state_entry {
		event next if ident { dst state_entry_next; act state_entry; }
	}
	state state_exit {
		event next if ident { dst state_exit_next; act state_exit; }
	}
	state state_start {
		event next if ident { dst state_start_next; act state_start; }
	}
	state {
		state state_name {
			event next if ident { dst state_name_next; act state_name; }
		}
		state state_name_next;
		event next if semi { dst state_next; act state_end; }
		event next if bra { dst state_next; }
	}
	state {
		state state_start_next;
		state state_entry_next {
			event next if comma { dst state_entry; }
		}
		state state_exit_next {
			event next if comma { dst state_exit; }
		}
		state state_next {
			event next if entry { dst state_entry; }
			event next if event { dst event_name; act event_begin; }
			event next if exit { dst state_exit; }
			event next if start { dst state_start; }
			event next if state { dst state_name; act state_begin; }
		}
		event next if semi { dst state_next; }
		event next if ket { dst state_next; act state_end; }
	}
	state event_name {
		event next if ident { dst event_name_next; act event_name; }
	}
	state event_cond {
		event next if ident { dst event_cond_next; act event_cond; }
	}
	state event_act {
		event next if ident { dst event_act_next; act event_act; }
	}
	state event_dst {
		event next if ident { dst event_dst_next; act event_dst; }
	}
	state {
		state event_name_next {
			event next if if { dst event_cond; }
		}
		state event_cond_next;
		event next if semi { dst state_next; act event_end; }
		event next if bra { dst event_next; }
	}
	state {
		state event_dst_next;
		state event_act_next {
			event next if comma { dst event_act; }
		}
		state event_next {
			event next if act { dst event_act; }
			event next if dst { dst event_dst; }
		}
		event next if semi { dst event_next; }
		event next if ket { dst state_next; act event_end; }
	}
	event next { act error_unexpected; }
}
*/
