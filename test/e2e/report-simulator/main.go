// The simulator is used only by the disposable Kind test.
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
)

func main() {
	var received atomic.Int64
	http.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int64{"received": received.Load()})
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			http.NotFound(w, r)
			return
		}
		received.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	_ = http.ListenAndServe(":8080", nil)
}
