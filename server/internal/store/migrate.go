package store

import "io/fs"

func mustSub() fs.FS {
	result, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err)
	}
	return result
}
