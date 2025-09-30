package bcode

import "net/http"

var (
	HealthCode = NewBcode(http.StatusOK, 80000, "health interface call success")
)
