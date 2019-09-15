// _example/main.go
package main

import (
	"fmt"
	"net/http"

	"github.com/brankas/goji"
)

func main() {
	m := goji.New()
	m.HandleFunc(goji.Get("/hello/:name"), func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, "hello %q!", goji.Param(req, "name"))
	})
	m.HandleFunc(goji.Get("/"), func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprint(res, "a page")
	})
	http.ListenAndServe(":3000", m)
}
