package smc

import (
	"io"
	"text/scanner"
)

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
