package smc

/**
smc.Parser {
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
**/

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
