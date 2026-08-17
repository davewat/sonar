package server

import (
	"bytes"
	_ "embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed web/index.html.tmpl
var indexHTMLSource string

var indexTmpl = template.Must(template.New("index").Parse(indexHTMLSource))

// renderIndex executes the diagnostic page template into a buffer first, so
// a template error can still fall back to a clean 500 response instead of
// leaving a half-written 200 on the wire.
func renderIndex(w http.ResponseWriter, data DiagnosticData) {
	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, data); err != nil {
		log.Printf("template render error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
}
