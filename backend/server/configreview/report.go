package configreview

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	dbmodel "isc.org/stork/server/database/model"
)

// Represents a single config review report. It contains a description
// of one issue found during a configuration review. The daemon field
// comprises an ID of the daemon for which the review is conducted.
// The refDaemons slice contain IDs of the daemons referenced in the
// review. Each daemon can be referenced at most once. The presence of
// the referenced daemons may trigger cascaded/internal reviews. See
// the dispatcher documentation.
type report struct {
	issue      string
	daemon     int64
	refDaemons []int64
}

// Represents an intermediate report which hasn't been validated yet.
type intermediateReport report

// Create new report. The report is associated with the subject daemon
// (a daemon for which the review is conducted) and includes an issue
// description. Additional functions can be called for this instance to
// add supplementary information to this report. For example, calling
// the referencingDaemon() function associates the report with the
// specified daemon. The report must not be used until create() function
// is called which sanity checks the report contents. An example usage:
//
// report, err := newReport(ctx, "some issue for {daemon} and {daemon}").
//     referencingDaemon(daemon1).
//     referencingDaemon(daemon2).
//     create()
//
// When the report is later fetched from the database it is possible to
// use the referenced daemons to replace the {daemon} placeholders with
// the detailed daemon information. See the similar mechanism implemented
// in the eventcenter.
func newReport(ctx *reviewContext, issue string) *intermediateReport {
	return &intermediateReport{
		issue:  strings.TrimSpace(issue),
		daemon: ctx.subjectDaemon.ID,
	}
}

// Associates a report with a daemon. Do not associate the same daemon
// with the report multiple times. It will result in an error while
// calling create().
func (r *intermediateReport) referencingDaemon(daemon *dbmodel.Daemon) *intermediateReport {
	r.refDaemons = append(r.refDaemons, daemon.ID)
	return r
}

// Validates the report contents and return an instance of the final
// report or an error. It should never report an error if the producers
// generating the reports are implemented properly.
func (r *intermediateReport) create() (*report, error) {
	// Ensure that the issue text is not blank.
	if len(r.issue) == 0 {
		return nil, pkgerrors.New("config review report must not be blank")
	}

	// Ensure that the subject daemon has non-zero ID.
	if r.daemon == 0 {
		return nil, pkgerrors.New("ID of the daemon for which a config report is created must not be 0")
	}

	// Ensure that each daemon is referenced at most once and it has
	// non-zero ID.
	presentDaemons := make(map[int64]bool)
	for _, id := range r.refDaemons {
		if id == 0 {
			return nil, pkgerrors.New("config review report must not reference a daemon with ID of 0")
		}
		if _, exists := presentDaemons[id]; exists {
			return nil, pkgerrors.Errorf("config review report must not reference the same daemon %d twice", id)
		}
		presentDaemons[id] = true
	}
	// Everything is fine.
	rc := &report{
		issue:      r.issue,
		daemon:     r.daemon,
		refDaemons: r.refDaemons,
	}
	return rc, nil
}