package dbops

import (
	"fmt"
	"strconv"

	"github.com/go-pg/migrations/v7"
	"github.com/go-pg/pg/v9"
	"github.com/pkg/errors"

	// TODO: document why it is blank imported.
	_ "isc.org/stork/server/database/migrations"
)

// Checks if the migrations table exists, i.e. the 'init' command was called.
func Initialized(db *PgDB) bool {
	var n int
	_, err := db.QueryOne(pg.Scan(&n), "SELECT count(*) FROM gopg_migrations")
	return err == nil
}

// Migrates the database version down to 0 and then removes the gopg_migrations
// table.
func Toss(db *PgDB) error {
	if db == nil {
		return errors.New("database is nil")
	}

	// Check if the migrations table exists. If it doesn't, there is nothing to do.
	if !Initialized(db) {
		return nil
	}

	// Migrate the database down to 0.
	_, _, err := Migrate(db, "reset")
	if err != nil {
		return err
	}

	// Remove the versioning table and id sequence.
	_, err = db.Exec(
		`DROP TABLE IF EXISTS gopg_migrations;
         DROP SEQUENCE IF EXISTS gopg_migrations_id_seq`)

	return err
}

// Migrates the database. The args specify one of the migration operations supported
// by go-pg/migrations. The returned arguments contain new and old database version as
// well as an error.
func Migrate(db *PgDB, args ...string) (oldVersion, newVersion int64, err error) {
	if len(args) > 0 && args[0] == "up" && !Initialized(db) {
		if oldVersion, newVersion, err = migrations.Run(db, "init"); err != nil {
			return oldVersion, newVersion, errors.Wrapf(err, "problem with initiating database")
		}
	}

	// If down migration was specified and specific version was specified, we need to do some tricks.
	// The migrations package doesn't allow migrating down to specific version, but it can migrate
	// down one step. So we can call it multiple times until it migrated down to the version we
	// want.
	if len(args) > 1 && args[0] == "down" {
		var oldVer int64
		if oldVer, _, err = migrations.Run(db, "version"); err != nil {
			return oldVer, oldVer, errors.Wrapf(err, "problem with checking database version")
		}
		toVer, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return oldVer, oldVer, errors.Wrapf(err, "can't parse -t argument %s as database version (expected integer)", args[1])
		}

		if toVer >= oldVer {
			return oldVer, oldVer, errors.New(fmt.Sprintf("Can't migrate down, current version %d, want to migrate to %d", oldVer, toVer))
		}
		startVer := oldVer
		var newVer int64 = 0
		for i := oldVer; i > toVer; i-- {
			if oldVer, newVer, err = migrations.Run(db, "down"); err != nil {
				return oldVer, oldVer, errors.Wrapf(err, "problem with checking database version")
			}
		}
		return startVer, newVer, nil
	}

	oldVersion, newVersion, err = migrations.Run(db, args...)
	if err != nil {
		return oldVersion, newVersion, errors.Wrapf(err, "problem with migrating database")
	}
	return oldVersion, newVersion, nil
}

// Migrates the database to the latest version. If the migrations are not initialized
// in the database, it also performs initialization step prior to running the
// migration.
func MigrateToLatest(db *PgDB) (oldVersion, newVersion int64, err error) {
	return Migrate(db, "up")
}

// Checks what is the highest available schema version.
func AvailableVersion() int64 {
	if regm := migrations.RegisteredMigrations(); len(regm) > 0 {
		return regm[len(regm)-1].Version
	}

	return 0
}

// Returns current schema version.
func CurrentVersion(db *PgDB) (int64, error) {
	return migrations.Version(db)
}
