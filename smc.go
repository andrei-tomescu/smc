package main

import (
	"bytes"
	"fmt"
	"io"
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
	if data, err := os.ReadFile(filename); err == nil {
		if bytes.Equal(text, data) {
			return
		}
	}
	if err := os.WriteFile(filename, text, 0666); err != nil {
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
	if len(os.Args) < 3 {
		panic("usage: smc [cs|cpp|go] <file>")
	}
	var (
		root *State
	)
	if data, err := os.ReadFile(os.Args[2]); err == nil {
		var text = string(data)
		var first = strings.Index(text, "/**") + 3
		var last = strings.Index(text, "**/")
		root = Scan(strings.NewReader(text[first:last]))
	} else {
		panic(err)
	}
	var src = PrintRoot(root, "")
	var buf = bytes.NewBuffer(nil)
	root.PushEvents()
	if os.Args[1] == "cs" {
		CodeGenCs(buf, root, src)
	}
	if os.Args[1] == "cpp" {
		CodeGenCpp(buf, root, src)
	}
	if os.Args[1] == "go" {
		CodeGenGo(buf, root, src)
	}
	if os.Args[1] == "lms-cs" {
		CodeGenCsLms(buf, root, src)
	}
	CheckWriteFile(os.Args[2], buf.Bytes())
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "using System;")
	line(0, "")
	line(0, "/**")
	line(0, strings.Join(source, "\r\n"))
	line(0, "**/")
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
				if empty(events) {
					continue
				}
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
}

func CodeGenCsLms(file io.Writer, root *State, source []string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("  ", idt))
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "using System;")
	line(0, "")
	line(0, "/**")
	line(0, strings.Join(source, "\r\n"))
	line(0, "**/")
	line(0, "")
	line(0, "namespace %s", strings.Join(ns, "."))
	line(0, "{")
	line(1, "public sealed class %s", name)
	line(1, "{")
	line(2, "public interface IHandler")
	line(2, "{")
	for _, cond := range allcond {
		line(3, "bool Cond%s();", Camel(cond))
	}
	for _, act := range allact {
		line(3, "void On%s();", Camel(act))
	}
	line(3, "void PostEvent(Action action);")
	line(2, "}")
	line(2, "")
	line(2, "public sealed class DelegateHandler: IHandler")
	line(2, "{")
	for _, cond := range allcond {
		line(3, "public bool Cond%s()", Camel(cond))
		line(3, "{")
		line(4, "return cond%s();", Camel(cond))
		line(3, "}")
		line(3, "public Func<bool> cond%s { get; set; }", Camel(cond))
	}
	for _, act := range allact {
		line(3, "public void On%s()", Camel(act))
		line(3, "{")
		line(4, "on%s();", Camel(act))
		line(3, "}")
		line(3, "public Action on%s { get; set; }", Camel(act))
	}
	line(3, "public void PostEvent(Action action)")
	line(3, "{")
	line(4, "postEvent(action);")
	line(3, "}")
	line(3, "public Action<Action> postEvent { get; set; }")
	line(2, "}")
	line(2, "")
	for _, ev := range allev {
		line(2, "public void Send%s()", Camel(ev))
		line(2, "{")
		line(3, "CurrentState.On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range allev {
		line(2, "public void Post%s()", Camel(ev))
		line(2, "{")
		line(3, "Handler.PostEvent(Send%s);", Camel(ev))
		line(2, "}")
	}
	line(2, "")
	line(2, "public void Start()")
	line(2, "{")
	line(3, "if (CurrentState == null)")
	line(3, "{")
	var actions, dst = MakeStart(root)
	for _, act := range actions {
		line(4, "Handler.On%s();", Camel(act))
	}
	line(4, "CurrentState = State%s.Instance;", Camel(dst.Name()))
	line(3, "}")
	line(2, "}")
	line(2, "")
	line(2, "private class IState")
	line(2, "{")
	for _, ev := range AllEvents(root) {
		line(3, "public virtual void On%s(%s parent)", Camel(ev), name)
		line(3, "{ }")
	}
	line(2, "}")
	line(2, "")
	line(2, "private class InvalidState: IState")
	line(2, "{")
	for _, ev := range AllEvents(root) {
		line(3, "public override void On%s(%s parent)", Camel(ev), name)
		line(3, "{")
		line(4, "throw new Exception();")
		line(3, "}")
	}
	line(3, "public static readonly IState Instance = new InvalidState();")
	line(2, "}")
	line(2, "")
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		line(2, "private class State%s: IState", Camel(state.Name()))
		line(2, "{")
		var groups = state.EventsGrouped()
		for _, evname := range allev {
			if events, found := groups[evname]; found {
				if empty(events) {
					continue
				}
				line(3, "public override void On%s(%s parent)", Camel(evname), name)
				line(3, "{")
				for _, event := range events {
					if event.HasCond() {
						line(4, "if (parent.Handler.Cond%s())", Camel(event.Cond()))
						line(4, "{")
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
		line(2, "")
	}
	line(2, "public %s(IHandler handler)", name)
	line(2, "{")
	line(3, "Handler = handler;")
	line(2, "}")
	line(2, "")
	line(2, "private readonly IHandler Handler;")
	line(2, "private IState CurrentState;")
	line(2, "")
	line(2, "public static void Test(Action<bool> assert)")
	line(2, "{")
	line(3, "var result = \"\";")
	line(3, "var handler = new DelegateHandler()")
	line(3, "{")
	for _, cond := range allcond {
		line(4, "cond%s = () => false,", Camel(cond))
	}
	for _, act := range allact {
		line(4, "on%s = () => result += \"<%s>\",", Camel(act), Camel(act))
	}
	line(3, "};")
	line(3, "var test = new %s(handler);", name)
	for _, state := range root.AllDescendants(root) {
		if state.IsNested() {
			continue
		}
		var groups = state.EventsGrouped()
		for _, evname := range allev {
			if events, found := groups[evname]; found {
				for _, event := range events {
					var actions, dst = MakeTransition(event)
					var message = ""
					for _, act := range actions {
						message += "<" + Camel(act) + ">"
					}
					line(3, "test.CurrentState = State%s.Instance;", Camel(state.Name()))
					if event.HasCond() {
						line(3, "handler.cond%s = () => true;", Camel(event.Cond()))
					}
					line(3, "test.Send%s();", Camel(evname))
					line(3, "assert(result == \"%s\");", message)
					if dst != nil {
						line(3, "assert(test.CurrentState == State%s.Instance);", Camel(dst.Name()))
					} else {
						line(3, "assert(test.CurrentState == State%s.Instance);", Camel(state.Name()))
					}
					if event.HasCond() {
						line(3, "handler.cond%s = () => false;", Camel(event.Cond()))
					}
					line(3, "result = \"\";")
				}
			}
		}
	}
	line(2, "}")
	line(1, "}")
	line(0, "}")
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

/******************************************************************************/

func CodeGenGo(file io.Writer, root *State, source []string) {
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
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
		parser *Parser
		scan   scanner.Scanner
		next   rune
		names  = make(map[string][]func(*State))
	)
	parser = &Parser{
		OnErrorUnexpected: func() {
			panic(scan.Pos().String() + ": unexpected " + scan.TokenText())
		},
		OnEventAct: func() {
			event.act = append(event.act, scan.TokenText())
		},
		OnEventBegin: func() {
			event = &Event{src: state}
		},
		OnEventCond: func() {
			event.cond = scan.TokenText()
		},
		OnEventDst: func() {
			var this = event
			var text = scan.TokenText()
			names[text] = append(names[text], func(st *State) {
				this.dst = st
			})
		},
		OnEventEnd: func() {
			if state.AddEvent(event) {
				panic(scan.Pos().String() + ": event " + event.Name() + " redeclared")
			}
		},
		OnEventName: func() {
			event.name = scan.TokenText()
		},
		OnRootBegin: func() {
			root = &State{name: scan.TokenText()}
			state = root
		},
		OnRootName: func() {
			root.name += "." + scan.TokenText()
		},
		OnStateBegin: func() {
			if state == nil {
				parser.OnErrorUnexpected()
			}
			state = &State{parent: state}
		},
		OnStateEnd: func() {
			if state == nil {
				parser.OnErrorUnexpected()
			}
			if state.parent != nil {
				state.parent.AddState(state)
			}
			state = state.parent
		},
		OnStateEntry: func() {
			state.entry = append(state.entry, scan.TokenText())
		},
		OnStateExit: func() {
			state.exit = append(state.exit, scan.TokenText())
		},
		OnStateName: func() {
			state.name = scan.TokenText()
		},
		OnStateStart: func() {
			var this = state
			var text = scan.TokenText()
			names[text] = append(names[text], func(st *State) {
				this.start = st
			})
		},
		CondAct: func() bool {
			return scan.TokenText() == "act"
		},
		CondBra: func() bool {
			return scan.TokenText() == "{"
		},
		CondComma: func() bool {
			return scan.TokenText() == ","
		},
		CondDot: func() bool {
			return scan.TokenText() == "."
		},
		CondDst: func() bool {
			return scan.TokenText() == "dst"
		},
		CondEntry: func() bool {
			return scan.TokenText() == "entry"
		},
		CondEvent: func() bool {
			return scan.TokenText() == "event"
		},
		CondExit: func() bool {
			return scan.TokenText() == "exit"
		},
		CondIdent: func() bool {
			return next == scanner.Ident
		},
		CondIf: func() bool {
			return scan.TokenText() == "if"
		},
		CondKet: func() bool {
			return scan.TokenText() == "}"
		},
		CondSemi: func() bool {
			return scan.TokenText() == ";"
		},
		CondStart: func() bool {
			return scan.TokenText() == "start"
		},
		CondState: func() bool {
			return scan.TokenText() == "state"
		},
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
	currentState      string
}

func (this *Parser) SendNext() {
	switch this.currentState {
	case "RootBegin":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnRootBegin()
			this.currentState = "RootNext"
			return
		}
		this.OnErrorUnexpected()
	case "RootNext":
		if this.CondBra() {
			this.currentState = "StateNext"
			return
		}
		if this.CondDot() {
			this.currentState = "RootName"
			return
		}
		this.OnErrorUnexpected()
	case "RootName":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnRootName()
			this.currentState = "RootNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateEntry":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnStateEntry()
			this.currentState = "StateEntryNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateExit":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnStateExit()
			this.currentState = "StateExitNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateStart":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnStateStart()
			this.currentState = "StateStartNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateName":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnStateName()
			this.currentState = "StateNameNext"
			return
		}
		if this.CondSemi() {
			this.currentState = "none"
			this.OnStateEnd()
			this.currentState = "StateNext"
			return
		}
		if this.CondBra() {
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateNameNext":
		if this.CondSemi() {
			this.currentState = "none"
			this.OnStateEnd()
			this.currentState = "StateNext"
			return
		}
		if this.CondBra() {
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateStartNext":
		if this.CondSemi() {
			this.currentState = "StateNext"
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnStateEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateEntryNext":
		if this.CondComma() {
			this.currentState = "StateEntry"
			return
		}
		if this.CondSemi() {
			this.currentState = "StateNext"
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnStateEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateExitNext":
		if this.CondComma() {
			this.currentState = "StateExit"
			return
		}
		if this.CondSemi() {
			this.currentState = "StateNext"
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnStateEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "StateNext":
		if this.CondEntry() {
			this.currentState = "StateEntry"
			return
		}
		if this.CondEvent() {
			this.currentState = "none"
			this.OnEventBegin()
			this.currentState = "EventName"
			return
		}
		if this.CondExit() {
			this.currentState = "StateExit"
			return
		}
		if this.CondStart() {
			this.currentState = "StateStart"
			return
		}
		if this.CondState() {
			this.currentState = "none"
			this.OnStateBegin()
			this.currentState = "StateName"
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
	case "EventName":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnEventName()
			this.currentState = "EventNameNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventCond":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnEventCond()
			this.currentState = "EventCondNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventAct":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnEventAct()
			this.currentState = "EventActNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventDst":
		if this.CondIdent() {
			this.currentState = "none"
			this.OnEventDst()
			this.currentState = "EventDstNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventNameNext":
		if this.CondIf() {
			this.currentState = "EventCond"
			return
		}
		if this.CondSemi() {
			this.currentState = "none"
			this.OnEventEnd()
			this.currentState = "StateNext"
			return
		}
		if this.CondBra() {
			this.currentState = "EventNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventCondNext":
		if this.CondSemi() {
			this.currentState = "none"
			this.OnEventEnd()
			this.currentState = "StateNext"
			return
		}
		if this.CondBra() {
			this.currentState = "EventNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventDstNext":
		if this.CondSemi() {
			this.currentState = "EventNext"
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnEventEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventActNext":
		if this.CondComma() {
			this.currentState = "EventAct"
			return
		}
		if this.CondSemi() {
			this.currentState = "EventNext"
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnEventEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "EventNext":
		if this.CondAct() {
			this.currentState = "EventAct"
			return
		}
		if this.CondDst() {
			this.currentState = "EventDst"
			return
		}
		if this.CondSemi() {
			return
		}
		if this.CondKet() {
			this.currentState = "none"
			this.OnEventEnd()
			this.currentState = "StateNext"
			return
		}
		this.OnErrorUnexpected()
	case "none":
		panic("invalid state")
	}
}
func (this *Parser) Start() {
	if this.currentState == "" {
		this.currentState = "RootBegin"
	}
}

/*
main.Parser {
	start RootBegin;
	state RootBegin {
		event Next if Ident { dst RootNext; act RootBegin; }
	}
	state RootNext {
		event Next if Bra { dst StateNext; }
		event Next if Dot { dst RootName; }
	}
	state RootName {
		event Next if Ident { dst RootNext; act RootName; }
	}
	state StateEntry {
		event Next if Ident { dst StateEntryNext; act StateEntry; }
	}
	state StateExit {
		event Next if Ident { dst StateExitNext; act StateExit; }
	}
	state StateStart {
		event Next if Ident { dst StateStartNext; act StateStart; }
	}
	state {
		state StateName {
			event Next if Ident { dst StateNameNext; act StateName; }
		}
		state StateNameNext;
		event Next if Semi { dst StateNext; act StateEnd; }
		event Next if Bra { dst StateNext; }
	}
	state {
		state StateStartNext;
		state StateEntryNext {
			event Next if Comma { dst StateEntry; }
		}
		state StateExitNext {
			event Next if Comma { dst StateExit; }
		}
		state StateNext {
			event Next if Entry { dst StateEntry; }
			event Next if Event { dst EventName; act EventBegin; }
			event Next if Exit { dst StateExit; }
			event Next if Start { dst StateStart; }
			event Next if State { dst StateName; act StateBegin; }
		}
		event Next if Semi { dst StateNext; }
		event Next if Ket { dst StateNext; act StateEnd; }
	}
	state EventName {
		event Next if Ident { dst EventNameNext; act EventName; }
	}
	state EventCond {
		event Next if Ident { dst EventCondNext; act EventCond; }
	}
	state EventAct {
		event Next if Ident { dst EventActNext; act EventAct; }
	}
	state EventDst {
		event Next if Ident { dst EventDstNext; act EventDst; }
	}
	state {
		state EventNameNext {
			event Next if If { dst EventCond; }
		}
		state EventCondNext;
		event Next if Semi { dst StateNext; act EventEnd; }
		event Next if Bra { dst EventNext; }
	}
	state {
		state EventDstNext;
		state EventActNext {
			event Next if Comma { dst EventAct; }
		}
		state EventNext {
			event Next if Act { dst EventAct; }
			event Next if Dst { dst EventDst; }
		}
		event Next if Semi { dst EventNext; }
		event Next if Ket { dst StateNext; act EventEnd; }
	}
	event Next { act ErrorUnexpected; }
}
*/
