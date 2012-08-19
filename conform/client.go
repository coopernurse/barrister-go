package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"io"
	"strings"
	"encoding/json"
	"github.com/coopernurse/barrister-go"
)

type ConformLine struct {
	iface      string
	function   string
	params     string
	exp_status string
	exp_result string
	act_status string
	act_result string
}

func (line *ConformLine) toActual() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", line.iface, line.function, line.params, line.act_status, line.act_result)
}

func (line *ConformLine) call(trans barrister.Transport) {

	method := fmt.Sprintf("%s.%s", line.iface, line.function)

	var params []interface{}
	err := json.Unmarshal([]byte(line.params), &params)
	if err != nil {
		panic(err)
	}

	res, rpcerr := trans.Call(method, params...)
	
	if rpcerr != nil {
		line.act_status = "rpcerr"
		line.act_result = fmt.Sprintf("%d", rpcerr.Code)
	} else {
		line.act_status = "ok"

		body, err := json.Marshal(res); if err != nil {
			panic(err)
		}
		line.act_result, err = barrister.EncodeASCII(body); if err != nil {
			panic(err)
		}
	}
}

func parseLine(s string) ConformLine {
	st := strings.TrimSpace(s)
	if len(st) > 0 && string(st[0]) != "#" {
		cols := strings.Split(s, "|")
		if len(cols) == 5 {
			return ConformLine{cols[0], cols[1], cols[2], cols[3], cols[4], "???", "???"}
		}
	}

	return ConformLine{}
}

func parseFile(fname string) ([]ConformLine, error) {
	var lines []ConformLine

	blank := ConformLine{}

	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(file)
	for {
		line, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		parsed := parseLine(string(line))
		if parsed != blank {
			lines = append(lines, parsed)
		}
	}
	file.Close()

	return lines, nil
}

func main() {
	flag.Parse()
	infile := flag.Arg(0)
	outfile := flag.Arg(1)

	lines, err := parseFile(infile)
	if err != nil {
		panic(err)
	}

	trans := &barrister.HttpTransport{"http://localhost:9233"}

	fo, err := os.Create(outfile)
    if err != nil { panic(err) }
    defer fo.Close()
    w := bufio.NewWriter(fo)

	for _, line := range lines {
		line.call(trans)
		w.WriteString(fmt.Sprintf("%s\n", line.toActual()))
	}
	w.Flush()
}