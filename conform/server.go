package main

import (
	"bytes"
	"fmt"
	"net/http"
	"github.com/coopernurse/barrister-go"
)

type B interface {
  // simply returns s 
  // if s == "return-null" then you should return a null 
  Echo(s string) *string 
}

type BImpl struct { }

func (b BImpl) Echo(s string) (*string, error) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

func main() {
	bimpl := BImpl{}
	
	svr := barrister.Server{}
	svr.AddHandler("B", bimpl)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		buf := bytes.Buffer{}
		_, err := buf.ReadFrom(r.Body); if err != nil {
			panic(err)
		}
		respJson := svr.InvokeJson(buf.Bytes())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, string(respJson))
	})

	err := http.ListenAndServe(":9233", nil); if err != nil {
		panic(err)
	}
}