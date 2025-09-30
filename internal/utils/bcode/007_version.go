package bcode

import "net/http"

var (
	VersionCode = NewBcode(http.StatusOK, 70000, "version interface call success")
)
