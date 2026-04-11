package main

import "fmt"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func versionString() string {
	return fmt.Sprintf("cpa-quota-inspector version=%s commit=%s built_at=%s", Version, Commit, BuildDate)
}
