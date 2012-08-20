package main

import (
	"flag"
	"bytes"
	"fmt"
	"math"
	"net/http"
	"io/ioutil"
	"strings"
	"github.com/coopernurse/barrister-go"
)

type Status string
var StatusOk = Status("ok")
var StatusErr = Status("err")


type MathOp string
var MathOpAdd = MathOp("add")
var MathOpMultiply = MathOp("multiply")

type Response struct {
    Status Status     `json:"status"`
}

// testing struct inheritance
type RepeatResponse struct {
	Status Status       `json:"status"`
    Count  int          `json:"count"`
    Items  []string     `json:"items"`
}

type HiResponse struct {
    Hi string      `json:"hi"`
}

type RepeatRequest struct {
    To_repeat        string    `json:"to_repeat"`
    Count            int       `json:"count"`
    Force_uppercase  bool      `json:"force_uppercase"`
}

type Person struct {
    PersonId  string       `json:"personId"`
    FirstName string       `json:"firstName"`
    LastName  string       `json:"lastName"`
    Email     *string      `json:"email"`
}

type A interface {
  // returns a+b
  Add(a int64, b int64) (int64, *barrister.JsonRpcError)

  // performs the given operation against 
  // all the values in nums and returns the result
  Calc(nums []float64, operation MathOp) (float64, *barrister.JsonRpcError)

  // returns the square root of a
  Sqrt(a float64) (float64, *barrister.JsonRpcError)

  // Echos the req1.to_repeat string as a list,
  // optionally forcing to_repeat to upper case
  //
  // RepeatResponse.items should be a list of strings
  // whose length is equal to req1.count
  Repeat(req1 RepeatRequest) (RepeatResponse, *barrister.JsonRpcError)

  //
  // returns a result with:
  //   hi="hi" and status="ok"
  Say_hi() (HiResponse, *barrister.JsonRpcError)

  // returns num as an array repeated 'count' number of times
  Repeat_num(num int64, count int64) ([]int64, *barrister.JsonRpcError)

  // simply returns p.personId
  //
  // we use this to test the '[optional]' enforcement, 
  // as we invoke it with a null email
  PutPerson(p Person) (string, *barrister.JsonRpcError)
}

type B interface {
	// simply returns s 
	// if s == "return-null" then you should return a null 
	Echo(s string) (*string, *barrister.JsonRpcError)
}

type AImpl struct { }

func (i AImpl) Add(a int64, b int64) (int64, *barrister.JsonRpcError) {
	return a+b, nil
}

func (a AImpl) Calc(nums []float64, operation MathOp) (float64, *barrister.JsonRpcError) {
	switch operation {
	case MathOpAdd:
		sum := float64(0)
		for i := 0; i < len(nums); i++ {
			sum += nums[i]
		}
		return sum, nil
	case MathOpMultiply:
		x := float64(1)
		for i := 0; i < len(nums); i++ {
			x = x * nums[i]
		}
		return x, nil
	}

	msg := fmt.Sprintf("Unknown operation: %s", operation)
	return 0, &barrister.JsonRpcError{Code: -32000, Message: msg}
}

  // returns the square root of a
func (i AImpl)  Sqrt(a float64) (float64, *barrister.JsonRpcError) {
	return math.Sqrt(a), nil
}

  // Echos the req1.to_repeat string as a list,
  // optionally forcing to_repeat to upper case
  //
  // RepeatResponse.items should be a list of strings
  // whose length is equal to req1.count
func (a AImpl)  Repeat(req1 RepeatRequest) (RepeatResponse, *barrister.JsonRpcError) {
	rr := RepeatResponse{Status:"ok", Count: req1.Count, Items: []string{}}

	s := req1.To_repeat
	if req1.Force_uppercase {
		s = strings.ToUpper(s)
	}
	for i := 0; i < req1.Count; i++ {
		rr.Items = append(rr.Items, s)
	}
	
	return rr, nil
}

	//
	// returns a result with:
  //   hi="hi" and status="ok"
func (a AImpl)  Say_hi() (HiResponse, *barrister.JsonRpcError) {
	return HiResponse{"hi"}, nil
}

  // returns num as an array repeated 'count' number of times
func (a AImpl)  Repeat_num(num int64, count int64) ([]int64, *barrister.JsonRpcError) {
	arr := []int64{}
	for i := int64(0); i < count; i++ {
		arr = append(arr, num)
	}
	return arr, nil
}

  // simply returns p.personId
  //
  // we use this to test the '[optional]' enforcement, 
  // as we invoke it with a null email
func (a AImpl)  PutPerson(p Person) (string, *barrister.JsonRpcError) {
	return p.PersonId, nil
}

type BImpl struct { }

func (b BImpl) Echo(s string) (*string, error) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

func main() {
	flag.Parse()
	idlFile := flag.Arg(0)

	b, err := ioutil.ReadFile(idlFile)
	if err != nil {
		panic(err)
	}

	idl, err := barrister.ParseIdlJson(b)
	if err != nil {
		panic(err)
	}

	svr := barrister.NewServer(idl)
	svr.AddHandler("A", AImpl{})
	svr.AddHandler("B", BImpl{})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		buf := bytes.Buffer{}
		_, err := buf.ReadFrom(r.Body); if err != nil {
			panic(err)
		}
		respJson := svr.InvokeJSON(buf.Bytes())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, string(respJson))
	})

	err = http.ListenAndServe(":9233", nil); if err != nil {
		panic(err)
	}
}