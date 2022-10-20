package directory

import (
	"encoding/json"
	"net/http"
)

func GetUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	encodeJSONError := json.NewEncoder(w).Encode("{}")
	if encodeJSONError != nil {
		http.Error(w, encodeJSONError.Error(), http.StatusBadRequest)
		return
	}
}
