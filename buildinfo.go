package main

import "fmt"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func versionString() string {
	return fmt.Sprintf("cpausage version=%s commit=%s built_at=%s", Version, Commit, BuildDate)
}
