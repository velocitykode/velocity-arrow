package store

import (
	"context"
	"fmt"

	"github.com/velocitykode/velocity/orm"
	"github.com/velocitykode/velocity/orm/drivers"
)

// snapshotDriver is the ORM driver name the knowledge-base snapshot is opened
// under. The built-in "sqlite" name is deliberately not reused: arrow
// blank-imports orm/standard, and with cgo enabled that leaf overrides
// "sqlite"/"sqlite3" with the mattn backend, which is compiled without the FTS5
// module. Every keyword query against the snapshot would then fail with
// "no such module: fts5".
const snapshotDriver = "velocity-kb-sqlite"

// init binds snapshotDriver to velocity's pure-Go modernc SQLite backend
// through the documented orm.Drivers() registration seam, so the snapshot keeps
// an FTS5-capable connection whatever else the process registers.
func init() {
	orm.Drivers().Register(snapshotDriver, func(_ context.Context, cfg drivers.ConnectionConfig) (drivers.Driver, error) {
		d := drivers.NewSQLiteDriver()
		if err := d.Connect(cfg); err != nil {
			return nil, fmt.Errorf("store: connect snapshot driver: %w", err)
		}
		return d, nil
	})
}
