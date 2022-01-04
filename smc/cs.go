package smc

import (
	"fmt"
	"io"
	"strings"
)

func PrintCs(file io.Writer, root *State, source []string) {
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
	var allcond = root.AllConditions()
	var allact = root.AllActions()
	var allev = root.AllEvents()
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
	for _, ev := range allev {
		line(3, "public virtual void On%s(%s parent) {", Camel(ev), name)
		line(3, "}")
	}
	line(2, "}")
	line(2, "private class InvalidState: IState {")
	for _, ev := range allev {
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

func PrintLmsCs(file io.Writer, root *State, source []string) {
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
	var allcond = root.AllConditions()
	var allact = root.AllActions()
	var allev = root.AllEvents()
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
	for _, ev := range allev {
		line(3, "public virtual void On%s(%s parent)", Camel(ev), name)
		line(3, "{ }")
	}
	line(2, "}")
	line(2, "")
	line(2, "private class InvalidState: IState")
	line(2, "{")
	for _, ev := range allev {
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
