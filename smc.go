package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"unicode"
)

/******************************************************************************/

type XEvent struct {
	Name string `xml:"id,attr"`
	Cond string `xml:"if,attr,omitempty"`
	Dst  string `xml:"dst,attr,omitempty"`
	Act  string `xml:"act,attr,omitempty"`
}

type XState struct {
	Name   string   `xml:"id,attr,omitempty"`
	Entry  string   `xml:"entry,attr,omitempty"`
	Exit   string   `xml:"exit,attr,omitempty"`
	Start  string   `xml:"start,attr,omitempty"`
	States []XState `xml:"state"`
	Events []XEvent `xml:"event"`
}

type XRoot struct {
	XMLName   xml.Name `xml:"fsm"`
	Namespace string   `xml:"ns,attr"`
	Name      string   `xml:"name,attr"`
	Start     string   `xml:"start,attr"`
	States    []XState `xml:"state"`
	Events    []XEvent `xml:"event"`
}

func (xroot XRoot) GetState() XState {
	return XState{
		States: xroot.States,
		Events: xroot.Events,
		Start:  xroot.Start,
	}
}

/******************************************************************************/

type XReader map[string][]func(*State)

func (this XReader) CreateState(xroot XState, parent *State) *State {
	var root = &State{
		name:   Trim(xroot.Name),
		parent: parent,
		entry:  SplitAttr(xroot.Entry),
		exit:   SplitAttr(xroot.Exit),
	}
	this.Add(Trim(xroot.Start), func(state *State) {
		root.start = state
	})
	for _, xchild := range xroot.States {
		root.AddState(this.CreateState(xchild, root))
	}
	for _, xevent := range xroot.Events {
		var event = &Event{
			name: Trim(xevent.Name),
			cond: Trim(xevent.Cond),
			src:  root,
			act:  SplitAttr(xevent.Act),
		}
		if root.AddEvent(event) {
			panic("event " + event.name + " redeclared in state " + root.name)
		}
		this.Add(Trim(xevent.Dst), func(state *State) {
			event.dst = state
		})
	}
	return root
}

func (this XReader) Add(name string, setfn func(*State)) {
	if name == "" {
		return
	}
	this[name] = append(this[name], setfn)
}

func (this XReader) Invoke(root *State) {
	for _, state := range root.AllDescendants(root) {
		if state.name == "" {
			continue
		}
		if list, ok := this[state.name]; ok {
			for _, setfn := range list {
				setfn(state)
			}
		}
		this[state.name] = nil
	}
	for name, list := range this {
		if len(list) != 0 {
			panic("unknown state " + name)
		}
	}
}

/******************************************************************************/

func ReadXml(file io.Reader) (*State, string, string, string) {
	var reader = make(XReader)
	var xroot XRoot
	var data, err = ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	err = xml.Unmarshal(data, &xroot)
	if err != nil {
		panic(err)
	}
	var root = reader.CreateState(xroot.GetState(), nil)
	reader.Invoke(root)
	var text, _ = xml.MarshalIndent(xroot, "", "\t")
	return root, xroot.Name, xroot.Namespace, string(text)
}

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
		panic("missing start in state " + state.Name())
	}
	if state.start.IsDescendantOf(state) {
		return state.start.FollowStart()
	}
	panic("invalid start in state " + state.Name())
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

func Trim(text string) string {
	return strings.TrimSpace(text)
}

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

func SplitAttr(text string) []string {
	var list []string
	for _, txt := range strings.Split(text, ",") {
		if Trim(txt) != "" {
			list = append(list, Trim(txt))
		}
	}
	return list
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
		panic("usage: smc [cs|cpp] <input-file> <output-file>")
	}
	var file, err = os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	var root, name, ns, src = ReadXml(file)
	root.PushEvents()
	if os.Args[1] == "cs" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCs(buffer, root, name, ns, src)
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
	if os.Args[1] == "cpp" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCpp(buffer, root, name, ns, src)
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
}

/******************************************************************************/

func CodeGenCs(file io.Writer, root *State, name, ns, source string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\n")
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "using System;")
	line(0, "")
	line(0, "namespace %s {", ns)
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
		line(3, "public Func<bool> cond%s {get; set;}", Camel(cond))
	}
	for _, act := range allact {
		line(3, "public void On%s() {", Camel(act))
		line(4, "on%s();", Camel(act))
		line(3, "}")
		line(3, "public Action on%s {get; set;}", Camel(act))
	}
	line(3, "public void PostEvent(Action action) {")
	line(4, "postEvent(action);")
	line(3, "}")
	line(3, "public Action<Action> postEvent {get; set;}")
	line(2, "}")
	for _, ev := range allev {
		line(2, "public void Send%s() {", Camel(ev))
		line(3, "CurrentState.On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range allev {
		line(2, "public void Post%s() {", Camel(ev))
		line(3, "Handler.PostEvent(() => Send%s());", Camel(ev))
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
	line(0, source)
	line(0, "*/")
}

/******************************************************************************/

func CodeGenCpp(file io.Writer, root *State, name, ns, source string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\n")
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
	var allcond = AllConditions(root)
	var allact = AllActions(root)
	var allev = AllEvents(root)
	line(0, "#pragma once")
	line(0, "")
	line(0, "namespace %s {", ns)
	line(1, "struct %s {", name)
	line(2, "using Event = void (%s::*)();", name)
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
	line(1, "protected:")
	for _, cond := range allcond {
		line(2, "virtual bool Cond%s() = 0;", Camel(cond))
	}
	for _, act := range allact {
		line(2, "virtual void On%s() = 0;", Camel(act))
	}
	line(2, "virtual void PostEvent(Event event) {")
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
	line(0, source)
	line(0, "*/")
}

/******************************************************************************/
