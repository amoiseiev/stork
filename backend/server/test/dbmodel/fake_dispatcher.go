package storktestdbmodel

import (
	"sort"

	"isc.org/stork/server/configreview"
	dbmodel "isc.org/stork/server/database/model"
)

// Represents a single call to FakeDispatcher.
type FakeDispatcherCall struct {
	CallName string
	DaemonID int64
	Trigger  configreview.Trigger
}

// Mock implementation of the configuration review dispatcher.
// It substitutes the default dispatcher implementation in the
// unit tests.
type FakeDispatcher struct {
	CallLog    []FakeDispatcherCall
	Signature  string
	InProgress bool
	// First key is the daemon ID. The global states use 0 index.
	// Second key is the checker name.
	checkerStates map[int64]map[string]configreview.CheckerState
}

var _ configreview.Dispatcher = (*FakeDispatcher)(nil)

func (d *FakeDispatcher) RegisterChecker(selector configreview.DispatchGroupSelector, checkerName string, triggers configreview.Triggers, checkFn func(*configreview.ReviewContext) (*configreview.Report, error)) {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "RegisterChecker"})
}

func (d *FakeDispatcher) UnregisterChecker(selector configreview.DispatchGroupSelector, checkerName string) bool {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "UnregisterChecker"})
	return true
}

func (d *FakeDispatcher) GetCheckersMetadata(daemon *dbmodel.Daemon) ([]*configreview.CheckerMetadata, error) {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "GetCheckersMetadata"})

	var daemonID int64
	if daemon != nil {
		daemonID = daemon.ID
	}

	var metadata []*configreview.CheckerMetadata
	if checkerStates, ok := d.checkerStates[daemonID]; ok {
		for checkerName, checkerState := range checkerStates {
			metadata = append(metadata, &configreview.CheckerMetadata{
				Name:  checkerName,
				State: checkerState,
			})
		}
	}

	// Sorts the metadata by name for the predictable order.
	sort.Slice(metadata, func(i, j int) bool {
		return metadata[i].Name < metadata[j].Name
	})
	return metadata, nil
}

func (d *FakeDispatcher) GetSignature() string {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "GetSignature"})
	return d.Signature
}

func (d *FakeDispatcher) SetCheckerState(daemon *dbmodel.Daemon, checkerName string, state configreview.CheckerState) error {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "SetCheckerState"})

	var daemonID int64
	if daemon != nil {
		daemonID = daemon.ID
	}

	// Initializes the intermediate maps.
	if d.checkerStates == nil {
		d.checkerStates = make(map[int64]map[string]configreview.CheckerState)
	}
	if _, ok := d.checkerStates[daemonID]; !ok {
		d.checkerStates[daemonID] = make(map[string]configreview.CheckerState)
	}

	if state == configreview.CheckerStateInherit || (daemon == nil && state == configreview.CheckerStateEnabled) {
		// Removes the inherited or default state from the map.
		delete(d.checkerStates[daemonID], checkerName)
	} else {
		d.checkerStates[daemonID][checkerName] = state
	}

	return nil
}

func (d *FakeDispatcher) Start() {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "Start"})
}

func (d *FakeDispatcher) Shutdown() {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "Shutdown"})
}

func (d *FakeDispatcher) BeginReview(daemon *dbmodel.Daemon, trigger configreview.Trigger, callback configreview.CallbackFunc) bool {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "BeginReview", DaemonID: daemon.ID, Trigger: trigger})
	return true
}

func (d *FakeDispatcher) ReviewInProgress(daemonID int64) bool {
	d.CallLog = append(d.CallLog, FakeDispatcherCall{CallName: "ReviewInProgress", DaemonID: daemonID})
	return d.InProgress
}
