package thingsv2

import (
	"net/http"

	"github.com/google/uuid"
)

// validThingID accepts the canonical UUID format used by things-v2 URLs.
func validThingID(id string) bool {
	return len(id) == 36 && uuid.Validate(id) == nil
}

func requireThingID(w http.ResponseWriter, id string) bool {
	if !validThingID(id) {
		http.Error(w, "id must be a UUID", http.StatusBadRequest)
		return false
	}
	return true
}
