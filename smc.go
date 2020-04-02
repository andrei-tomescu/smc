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

type XmlEvent struct {
	Name string `xml:"id,attr"`
	Cond string `xml:"if,attr,omitempty"`
	Dst  string `xml:"dst,attr,omitempty"`
	Act  string `xml:"act,attr,omitempty"`
}

type XmlState struct {
	Name   string     `xml:"id,attr"`
	Entry  string     `xml:"entry,attr,omitempty"`
	Exit   string     `xml:"exit,attr,omitempty"`
	Start  string     `xml:"start,attr,omitempty"`
	States []XmlState `xml:"state"`
	Events []XmlEvent `xml:"event"`
}

type XmlRoot struct {
	XMLName   xml.Name   `xml:"fsm"`
	Namespace string     `xml:"ns,attr"`
	Name      string     `xml:"name,attr"`
	Start     string     `xml:"start,attr"`
	States    []XmlState `xml:"state"`
	Events    []XmlEvent `xml:"event"`
}

func (this XmlRoot) GetXmlState() XmlState {
	return XmlState{
		States: this.States,
		Events: this.Events,
		Name:   this.Name,
		Start:  this.Start,
	}
}

/******************************************************************************/

func SplitAttr(text string) []string {
	var list = make([]string, 0)
	for _, txt := range strings.Split(text, ",") {
		if TrimName(txt) != "" {
			list = append(list, TrimName(txt))
		}
	}
	return list
}

func TrimName(text string) string {
	return strings.TrimSpace(text)
}

func StateFromXml(xmlroot XmlState, parent IState, resolve map[string][]func(IState)) IState {
	var state = &State{
		name:   TrimName(xmlroot.Name),
		parent: parent,
		entry:  SplitAttr(xmlroot.Entry),
		exit:   SplitAttr(xmlroot.Exit),
	}
	for _, xmlst := range xmlroot.States {
		state.nested = append(state.nested, StateFromXml(xmlst, state, resolve))
	}
	for _, xmlev := range xmlroot.Events {
		var event = &Event{
			name: TrimName(xmlev.Name),
			cond: TrimName(xmlev.Cond),
			src:  state,
			act:  SplitAttr(xmlev.Act),
		}
		var dst = TrimName(xmlev.Dst)
		resolve[dst] = append(resolve[dst], func(st IState) {
			event.dst = st
		})
		state.events = append(state.events, event)
	}
	var start = TrimName(xmlroot.Start)
	resolve[start] = append(resolve[start], func(st IState) {
		state.start = st
	})
	return state
}

func FromXml(xmlroot XmlState) IState {
	var resolve = make(map[string][]func(IState))
	var root = StateFromXml(xmlroot, nil, resolve)
	ForEachState(root, func(st IState) {
		if st.Name() == "" {
			return
		}
		if list, ok := resolve[st.Name()]; ok {
			for _, fn := range list {
				fn(st)
			}
			resolve[st.Name()] = nil
		}
	})
	for key, val := range resolve {
		if key != "" && val != nil {
			panic("unknown state " + key)
		}
	}
	return root
}

func ReadXml(file io.Reader) (root XmlRoot) {
	data, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	err = xml.Unmarshal(data, &root)
	if err != nil {
		panic(err)
	}
	return
}

func ToXml(xmlroot XmlRoot, prefix string) []byte {
	data, err := xml.MarshalIndent(xmlroot, prefix, "  ")
	if err != nil {
		panic(err)
	}
	return data
}

/******************************************************************************/

type IState interface {
	Name() string
	Start() IState
	Parent() IState
	Entry() []string
	Exit() []string
	Children() []IState
	Events() []IEvent
	IsNested() bool
	IsLeaf() bool
	AddEvent(IEvent)
}

type IEvent interface {
	Name() string
	Cond() string
	Src() IState
	Dst() IState
	Actions() []string
	IsInternal() bool
}

type IAction interface {
	IsFunc() bool
	IsState() bool
	Func() string
	State() IState
}

/******************************************************************************/

type State struct {
	name   string
	start  IState
	parent IState
	entry  []string
	exit   []string
	nested []IState
	events []IEvent
}

func (state *State) Name() string {
	return state.name
}
func (state *State) Start() IState {
	return state.start
}
func (state *State) Parent() IState {
	return state.parent
}
func (state *State) Entry() []string {
	return state.entry
}
func (state *State) Exit() []string {
	return state.exit
}
func (state *State) Children() []IState {
	return state.nested
}
func (state *State) Events() []IEvent {
	return state.events
}
func (state *State) IsNested() bool {
	return len(state.nested) != 0
}
func (state *State) IsLeaf() bool {
	return len(state.nested) == 0
}
func (state *State) AddEvent(event IEvent) {
	for _, ev := range state.events {
		if ev.Name() == event.Name() {
			if ev.Cond() == event.Cond() {
				return
			}
		}
	}
	state.events = append(state.events, event)
}

/******************************************************************************/

type Event struct {
	name string
	cond string
	src  IState
	dst  IState
	act  []string
}

func (event *Event) Name() string {
	return event.name
}
func (event *Event) Cond() string {
	return event.cond
}
func (event *Event) Src() IState {
	return event.src
}
func (event *Event) Dst() IState {
	return event.dst
}
func (event *Event) Actions() []string {
	return event.act
}
func (event *Event) IsInternal() bool {
	return event.dst == nil
}

/******************************************************************************/

type Action struct {
	fn string
	st IState
}

func (act *Action) IsFunc() bool {
	return act.fn != ""
}
func (act *Action) IsState() bool {
	return act.st != nil
}
func (act *Action) Func() string {
	return act.fn
}
func (act *Action) State() IState {
	return act.st
}
func NewActionFunc(fn string) IAction {
	return &Action{
		fn: fn,
	}
}
func NewActionState(state IState) IAction {
	return &Action{
		st: state,
	}
}

/******************************************************************************/

func ResolveEvents(root IState) {
	ForEachChild(root, false, func(state IState) {
		ResolveEvents(state)
	})
	if root.IsNested() {
		ForEachEvent(root, func(event IEvent) {
			ForEachChild(root, true, func(state IState) {
				state.AddEvent(&Event{
					event.Name(),
					event.Cond(),
					state,
					event.Dst(),
					event.Actions(),
				})
			})
		})
	}
}

func Validate(root IState) {
	var states = make(map[string]bool)
	ForEachState(root, func(state IState) {
		if state.Name() == "" {
			return
		}
		if _, found := states[state.Name()]; found {
			panic("duplicate state id " + state.Name())
		}
		states[state.Name()] = true
	})
	ForEachState(root, func(state IState) {
		for idx1, event1 := range state.Events() {
			for idx2, event2 := range state.Events() {
				if idx1 == idx2 {
					continue
				}
				if event1.Name() != event2.Name() {
					continue
				}
				if event1.Cond() != event2.Cond() {
					continue
				}
				panic("duplicate event " + event1.Name() + " in state " + state.Name())
			}
		}
	})
}

/******************************************************************************/

func MakeEntry(path ...IState) (actions []IAction) {
	if len(path) == 0 {
		return
	}
	for _, act := range path[0].Entry() {
		actions = append(actions, NewActionFunc(act))
	}
	if path[0].IsLeaf() {
		actions = append(actions, NewActionState(path[0]))
		return
	}
	if path[0].IsNested() {
		actions = append(actions, MakeEntry(path[1:]...)...)
		return
	}
	panic("wtf")
}

func MakeExit(path ...IState) (actions []IAction) {
	if len(path) == 0 {
		return
	}
	for _, act := range path[0].Exit() {
		actions = append(actions, NewActionFunc(act))
	}
	actions = append(MakeExit(path[1:]...), actions...)
	return
}

func MakeInternal(event IEvent) (actions []IAction) {
	for _, act := range event.Actions() {
		actions = append(actions, NewActionFunc(act))
	}
	return
}

func MakeTransition(event IEvent) (actions []IAction) {
	if event.IsInternal() {
		return MakeInternal(event)
	}
	var src, dst = event.Src(), event.Dst()
	for dst.IsNested() {
		if dst.Start() == nil {
			panic("undefined start, unable to transition to " + dst.Name())
		}
		dst = dst.Start()
	}
	var expath, enpath = Diff(Path(src), Path(dst))
	actions = append(actions, MakeExit(expath...)...)
	actions = append(actions, MakeInternal(event)...)
	actions = append(actions, MakeEntry(enpath...)...)
	return
}

func MakeInit(root IState) []IAction {
	for root.IsNested() {
		if root.Start() == nil {
			panic("undefined start, unable to transition to " + root.Name())
		}
		root = root.Start()
	}
	return MakeEntry(Path(root)...)
}

func Path(root IState) []IState {
	if root == nil {
		return nil
	}
	return append(Path(root.Parent()), root)
}

func Diff(src, dst []IState) ([]IState, []IState) {
	if len(src) == 0 || len(dst) == 0 || src[0] != dst[0] {
		return src, dst
	}
	return Diff(src[1:], dst[1:])
}

/******************************************************************************/

func ForEachChild(root IState, recursive bool, fn func(IState)) {
	for _, state := range root.Children() {
		fn(state)
		if recursive {
			ForEachChild(state, recursive, fn)
		}
	}
}

func ForEachState(root IState, fn func(IState)) {
	fn(root)
	ForEachChild(root, true, fn)
}

func ForEachEvent(root IState, fn func(IEvent)) {
	for _, event := range root.Events() {
		fn(event)
	}
}

func ForEachEventGrouped(root IState, fn func(string, []IEvent)) {
	var evlist []string
	var evdict = make(map[string][]IEvent)
	ForEachEvent(root, func(event IEvent) {
		evlist = append(evlist, event.Name())
	})
	evlist = StringSet(evlist)
	ForEachEvent(root, func(event IEvent) {
		if event.Cond() != "" {
			evdict[event.Name()] = append(evdict[event.Name()], event)
		}
	})
	ForEachEvent(root, func(event IEvent) {
		if event.Cond() == "" {
			evdict[event.Name()] = append(evdict[event.Name()], event)
		}
	})
	for _, event := range evlist {
		fn(event, evdict[event])
	}
}

/******************************************************************************/

func AllEvents(root IState) []string {
	var events []string
	ForEachState(root, func(state IState) {
		ForEachEvent(state, func(event IEvent) {
			events = append(events, event.Name())
		})
	})
	return StringSet(events)
}

func AllActions(root IState) []string {
	var actions []string
	ForEachState(root, func(state IState) {
		ForEachEvent(state, func(event IEvent) {
			actions = append(actions, event.Actions()...)
		})
		actions = append(actions, state.Entry()...)
		actions = append(actions, state.Exit()...)
	})
	return StringSet(actions)
}

func AllConditions(root IState) []string {
	var conditions []string
	ForEachState(root, func(state IState) {
		ForEachEvent(state, func(event IEvent) {
			if event.Cond() != "" {
				conditions = append(conditions, event.Cond())
			}
		})
	})
	return StringSet(conditions)
}

/******************************************************************************/

func StringSet(list []string) (set []string) {
	sort.Strings(list)
	for idx, item := range list {
		if idx != 0 && item == list[idx-1] {
			continue
		}
		set = append(set, item)
	}
	return
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
		}
	}()
	if len(os.Args) != 4 {
		panic("usage: smc [cs|cpp] <input-file> <output-file>")
	}
	file, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	xmlroot := ReadXml(file)
	fsmroot := FromXml(xmlroot.GetXmlState())
	Validate(fsmroot)
	ResolveEvents(fsmroot)
	if os.Args[1] == "cs" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCs(buffer, fsmroot, fsmroot.Name(), xmlroot.Namespace)
		buffer.Write(ToXml(xmlroot, "// "))
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
	if os.Args[1] == "cpp" {
		var buffer = bytes.NewBuffer(nil)
		CodeGenCpp(buffer, fsmroot, fsmroot.Name(), xmlroot.Namespace)
		buffer.Write(ToXml(xmlroot, "// "))
		CheckWriteFile(os.Args[3], buffer.Bytes())
	}
}

/******************************************************************************/

func CodeGenCs(file io.Writer, root IState, name, ns string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\n")
	}
	var transition = func(idt int, event IEvent) {
		var actions = MakeTransition(event)
		for _, act := range actions {
			if act.IsState() {
				line(idt, "parent.CurrentState = InvalidState.Instance;")
				break
			}
		}
		for _, act := range actions {
			if act.IsFunc() {
				line(idt, "parent.Handler.On%s();", Camel(act.Func()))
			}
			if act.IsState() {
				line(idt, "parent.CurrentState = State%s.Instance;", Camel(act.State().Name()))
			}
		}
	}
	line(0, "using System;")
	line(0, "")
	line(0, "namespace %s {", ns)
	line(1, "public class %s {", name)
	line(2, "public interface IEventHandler {")
	for _, cond := range AllConditions(root) {
		line(3, "bool Cond%s();", Camel(cond))
	}
	for _, act := range AllActions(root) {
		line(3, "void On%s();", Camel(act))
	}
	line(3, "void PostEvent(Action action);")
	line(2, "}")
	for _, ev := range AllEvents(root) {
		line(2, "public void Send%s() {", Camel(ev))
		line(3, "CurrentState.On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range AllEvents(root) {
		line(2, "public void Post%s() {", Camel(ev))
		line(3, "Handler.PostEvent(() => Send%s());", Camel(ev))
		line(2, "}")
	}
	line(2, "public void Start() {")
	line(3, "if (CurrentState == null) {")
	for _, act := range MakeInit(root) {
		if act.IsFunc() {
			line(4, "Handler.On%s();", Camel(act.Func()))
		}
		if act.IsState() {
			line(4, "CurrentState = State%s.Instance;", Camel(act.State().Name()))
		}
	}
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
	ForEachState(root, func(state IState) {
		if state.IsLeaf() {
			line(2, "private class State%s: IState {", Camel(state.Name()))
			ForEachEventGrouped(state, func(evname string, events []IEvent) {
				line(3, "public override void On%s(%s parent) {", Camel(evname), name)
				for _, event := range events {
					if event.Cond() != "" {
						line(4, "if (parent.Handler.Cond%s()) {", Camel(event.Cond()))
						transition(5, event)
						line(5, "return;")
						line(4, "}")
					} else {
						transition(4, event)
					}
				}
				line(3, "}")
			})
			line(3, "public static readonly IState Instance = new State%s();", Camel(state.Name()))
			line(2, "}")
		}
	})
	line(2, "public %s(IEventHandler handler) {", name)
	line(3, "Handler = handler;")
	line(2, "}")
	line(2, "private readonly IEventHandler Handler;")
	line(2, "private IState CurrentState;")
	line(1, "}")
	line(0, "}")
	line(0, "")
}

/******************************************************************************/

func CodeGenCpp(file io.Writer, root IState, name, ns string) {
	var line = func(idt int, format string, args ...interface{}) {
		fmt.Fprintf(file, strings.Repeat("\t", idt))
		fmt.Fprintf(file, format, args...)
		fmt.Fprintf(file, "\n")
	}
	var transition = func(idt int, event IEvent) {
		var actions = MakeTransition(event)
		for _, act := range actions {
			if act.IsState() {
				line(idt, "parent->SetInvalidState();")
				break
			}
		}
		for _, act := range actions {
			if act.IsFunc() {
				line(idt, "parent->On%s();", Camel(act.Func()))
			}
			if act.IsState() {
				line(idt, "parent->SetState%s();", Camel(act.State().Name()))
			}
		}
	}
	line(0, "#pragma once")
	line(0, "")
	line(0, "namespace %s {", ns)
	line(1, "struct %s {", name)
	line(2, "using Event = void (%s::*)();", name)
	for _, ev := range AllEvents(root) {
		line(2, "void Send%s() {", Camel(ev))
		line(3, "CurrentState->On%s(this);", Camel(ev))
		line(2, "}")
	}
	for _, ev := range AllEvents(root) {
		line(2, "void Post%s() {", Camel(ev))
		line(3, "PostEvent(&%s::Send%s);", name, Camel(ev))
		line(2, "}")
	}
	line(2, "void Start() {")
	line(3, "if (CurrentState == nullptr) {")
	for _, act := range MakeInit(root) {
		if act.IsFunc() {
			line(4, "On%s();", Camel(act.Func()))
		}
		if act.IsState() {
			line(4, "SetState%s();", Camel(act.State().Name()))
		}
	}
	line(3, "}")
	line(2, "}")
	line(1, "protected:")
	for _, cond := range AllConditions(root) {
		line(2, "virtual bool Cond%s() = 0;", Camel(cond))
	}
	for _, act := range AllActions(root) {
		line(2, "virtual void On%s() = 0;", Camel(act))
	}
	line(2, "virtual void PostEvent(Event event) {")
	line(2, "}")
	line(2, "void ProcessEvent(Event event) {")
	line(3, "(this->*event)();")
	line(2, "}")
	line(1, "private:")
	line(2, "struct IState {")
	for _, ev := range AllEvents(root) {
		line(3, "virtual void On%s(%s *) {", Camel(ev), name)
		line(3, "}")
	}
	line(2, "};")
	line(2, "struct InvalidState: IState {")
	for _, ev := range AllEvents(root) {
		line(3, "void On%s(%s *) override {", Camel(ev), name)
		line(4, "throw \"invalid state\";")
		line(3, "}")
	}
	line(2, "};")
	ForEachState(root, func(state IState) {
		if state.IsLeaf() {
			line(2, "struct State%s: IState {", Camel(state.Name()))
			ForEachEventGrouped(state, func(evname string, events []IEvent) {
				line(3, "void On%s(%s *parent) override {", Camel(evname), name)
				for _, event := range events {
					if event.Cond() != "" {
						line(4, "if (parent->Cond%s()) {", Camel(event.Cond()))
						transition(5, event)
						line(5, "return;")
						line(4, "}")
					} else {
						transition(4, event)
					}
				}
				line(3, "}")
			})
			line(2, "};")
		}
	})
	line(2, "void SetInvalidState() {")
	line(3, "static InvalidState Instance;")
	line(3, "CurrentState = &Instance;")
	line(2, "}")
	ForEachState(root, func(state IState) {
		if state.IsNested() {
			return
		}
		line(2, "void SetState%s() {", Camel(state.Name()))
		line(3, "static State%s Instance;", Camel(state.Name()))
		line(3, "CurrentState = &Instance;")
		line(2, "}")
	})
	line(2, "IState *CurrentState = nullptr;")
	line(1, "};")
	line(0, "}")
	line(0, "")
}

/******************************************************************************/
