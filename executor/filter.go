package executor

import (
	"github.com/openmetric/yamf/internal/types"
	"sync"
)

type eventFilter struct {
	// Filter modes
	// 0: filter nothing
	// 1: only fire on status change
	// 2: fire all non-ok events, but only first ok events
	mode int

	lastStatus map[string]int

	sync.RWMutex

	ShouldEmit func(*types.Event) bool
}

func NewEventFilter(mode int) (*eventFilter, error) {
	f := &eventFilter{
		mode:       mode,
		lastStatus: make(map[string]int),
	}
	switch mode {
	case 0:
		f.ShouldEmit = f.shouldEmit0
	case 1:
		f.ShouldEmit = f.shouldEmit1
	case 2:
		f.ShouldEmit = f.shouldEmit2
	}
	return f, nil
}

func (f *eventFilter) shouldEmit0(e *types.Event) bool {
	// mode 0, filter nothing, just return true
	return true
}

func (f *eventFilter) shouldEmit1(e *types.Event) bool {
	// mode 1, only fire on status change
	f.Lock()
	defer f.Unlock()

	if last, ok := f.lastStatus[e.Identifier]; ok {
		if last != e.Status {
			f.lastStatus[e.Identifier] = e.Status
			return true
		}
	} else {
		// first seen, fire if not ok
		f.lastStatus[e.Identifier] = e.Status
		if e.Status != types.OK {
			return true
		}
	}

	return false
}

func (f *eventFilter) shouldEmit2(e *types.Event) bool {
	// 2: fire all non-ok events, but only first ok events
	f.Lock()
	defer f.Unlock()

	if last, ok := f.lastStatus[e.Identifier]; ok {
		// cases to fire:
		//  * last != current
		//  * last == current && current != OK
		if last != e.Status {
			f.lastStatus[e.Identifier] = e.Status
			return true
		} else if e.Status != types.OK {
			return true
		}
	} else {
		// first seen, fire if not ok
		f.lastStatus[e.Identifier] = e.Status
		if e.Status != types.OK {
			return true
		}
	}

	return false
}
