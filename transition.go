package smc

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
