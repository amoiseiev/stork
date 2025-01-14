package eventcenter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbmodel "isc.org/stork/server/database/model"
	dbtest "isc.org/stork/server/database/test"
)

// Check creating event.
func TestCreateEvent(t *testing.T) {
	app := &dbmodel.App{
		ID:   123,
		Type: dbmodel.AppTypeKea,
		Name: "dhcp-server",
		Meta: dbmodel.AppMeta{
			Version: "1.2.3",
		},
	}
	daemon := &dbmodel.Daemon{
		ID:    234,
		Name:  "dhcp4",
		App:   app,
		AppID: app.ID,
	}
	subnet := &dbmodel.Subnet{
		ID:     345,
		Prefix: "192.0.0.0/8",
	}
	machine := &dbmodel.Machine{
		ID: 456,
	}
	user := &dbmodel.SystemUser{
		ID:    567,
		Login: "login",
		Email: "email",
	}

	// warning event with ref to app
	ev := CreateEvent(dbmodel.EvWarning, "some text {app} and {user}", app, user)
	require.EqualValues(t, ev.Text, "some text <app id=\"123\" name=\"dhcp-server\" type=\"kea\" version=\"1.2.3\"> and <user id=\"567\" login=\"login\" email=\"email\">")
	require.EqualValues(t, dbmodel.EvWarning, ev.Level)
	require.NotNil(t, ev.Relations)
	require.EqualValues(t, 123, ev.Relations.AppID)

	// info event with ref to app and daemon
	ev = CreateEvent(dbmodel.EvInfo, "some {daemon} text", daemon, app)
	require.EqualValues(t, "some <daemon id=\"234\" name=\"dhcp4\" appId=\"123\" appType=\"kea\"> text", ev.Text)
	require.EqualValues(t, dbmodel.EvInfo, ev.Level)
	require.NotNil(t, ev.Relations)
	require.EqualValues(t, 123, ev.Relations.AppID)
	require.EqualValues(t, 234, ev.Relations.DaemonID)

	// error event with ref to machine and subnet
	ev = CreateEvent(dbmodel.EvError, "some {subnet} text {machine}", daemon, app, subnet, machine)
	require.EqualValues(t, "some <subnet id=\"345\" prefix=\"192.0.0.0/8\"> text <machine id=\"456\" address=\"\" hostname=\"\">", ev.Text)
	require.EqualValues(t, dbmodel.EvError, ev.Level)
	require.NotNil(t, ev.Relations)
	require.EqualValues(t, 123, ev.Relations.AppID)
	require.EqualValues(t, 234, ev.Relations.DaemonID)
	require.EqualValues(t, 345, ev.Relations.SubnetID)
	require.EqualValues(t, 456, ev.Relations.MachineID)
}

// Check adding event.
func TestAddEvent(t *testing.T) {
	db, _, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	ec := NewEventCenter(db)

	app := &dbmodel.App{
		ID:   123,
		Type: dbmodel.AppTypeKea,
		Meta: dbmodel.AppMeta{
			Version: "1.2.3",
		},
	}
	daemon := &dbmodel.Daemon{
		Name:  "dhcp4",
		App:   app,
		AppID: app.ID,
	}
	subnet := &dbmodel.Subnet{
		ID: 345,
	}
	machine := &dbmodel.Machine{
		ID: 456,
	}

	ec.AddInfoEvent("some text", daemon, app)
	ec.AddWarningEvent("some text", subnet, app)
	ec.AddErrorEvent("some text", daemon, machine)

	// events are stored in db in separate goroutine so it may be delay
	// so wait for it a little bit
	var events []dbmodel.Event
	var total int64
	var err error

	require.Eventually(t, func() bool {
		events, total, err = dbmodel.GetEventsByPage(db, 0, 10, 0, nil, nil, nil, nil, "", dbmodel.SortDirAny)
		return total >= 3
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, events, 3)
	require.EqualValues(t, "some text", events[0].Text)
}
