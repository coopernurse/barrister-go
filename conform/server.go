package main

import (
	"flag"
	"fmt"
	"github.com/coopernurse/barrister-go"
	. "github.com/coopernurse/barrister-go/conform/generated"
	"math"
	"net/http"
	"strings"
)

type AImpl struct{}

func (i AImpl) Add(a int64, b int64) (int64, error) {
	return a + b, nil
}

func (a AImpl) Calc(nums []float64, operation MathOp) (float64, error) {
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
func (i AImpl) Sqrt(a float64) (float64, error) {
	return math.Sqrt(a), nil
}

// Echos the req1.to_repeat string as a list,
// optionally forcing to_repeat to upper case
//
// RepeatResponse.items should be a list of strings
// whose length is equal to req1.count
func (a AImpl) Repeat(req1 RepeatRequest) (RepeatResponse, error) {
	rr := RepeatResponse{Status: "ok", Count: req1.Count, Items: []string{}}

	s := req1.To_repeat
	if req1.Force_uppercase {
		s = strings.ToUpper(s)
	}
	for i := int64(0); i < req1.Count; i++ {
		rr.Items = append(rr.Items, s)
	}

	return rr, nil
}

//
// returns a result with:
//   hi="hi" and status="ok"
func (a AImpl) Say_hi() (HiResponse, error) {
	return HiResponse{"hi"}, nil
}

// returns num as an array repeated 'count' number of times
func (a AImpl) Repeat_num(num int64, count int64) ([]int64, error) {
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
func (a AImpl) PutPerson(p Person) (string, error) {
	return p.PersonId, nil
}

type BImpl struct{}

func (b BImpl) Echo(s string) (*string, error) {
	if s == "return-null" {
		return nil, nil
	}
	return &s, nil
}

func main() {
	flag.Parse()
	idlFile := flag.Arg(0)

	idl, err := barrister.ParseIdlJsonFile(idlFile)
	if err != nil {
		panic(err)
	}

	svr := NewJSONServer(idl, true, AImpl{}, BImpl{})
	http.Handle("/", &svr)

	err = http.ListenAndServe(":9233", nil)
	if err != nil {
		panic(err)
	}
}
