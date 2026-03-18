package controller

import (
	"embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPIFS embed.FS

func HandleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	b, _ := openAPIFS.ReadFile("openapi.yaml")
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Write(b)
}
