package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func encodeASCII(b []byte) (string, error) {
	in := bytes.NewBuffer(b)
	out := bytes.NewBufferString("")
	for {
		r, size, err := in.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if size == 1 {
			out.WriteRune(r)
		} else if size == 2 {
			out.WriteString(fmt.Sprintf("\\u%04x", r))
		} else {
			out.WriteString(fmt.Sprintf("\\U%08x", r))
		}
	}
	return out.String(), nil
}

type JsonRpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type JsonRpcResponse struct {
	Error  JsonRpcError `json:"error"`
	Result interface{}  `json:"result"`
}

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

func (line *ConformLine) call(url string) {
	req := fmt.Sprintf(`{"jsonrpc":"2.0", "id":"1234", "method":"%s.%s", "params":%s}`, line.iface, line.function, line.params)

	//fmt.Printf("   req: %s\n", req)
	buf := bytes.NewBufferString(req)
	resp, err := http.Post(url, "application/json", buf)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body); if err != nil {
		panic(err)
	}

	//fmt.Printf("result: %s\n", string(body))

	rpcResp := JsonRpcResponse{}
	err = json.Unmarshal(body, &rpcResp); if err != nil {
		panic(err)
	}
	
	if rpcResp.Error.Code != 0 && rpcResp.Result == nil {
		line.act_status = "rpcerr"
		line.act_result = fmt.Sprintf("%d", rpcResp.Error.Code)
	} else {
		line.act_status = "ok"

		body, err = json.Marshal(rpcResp.Result); if err != nil {
			panic(err)
		}
		line.act_result, err = encodeASCII(body); if err != nil {
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
	//fmt.Printf("infile: %s\n", infile)

	lines, err := parseFile(infile)
	if err != nil {
		panic(err)
	}

	for _, line := range lines {
		line.call("http://localhost:9233")
		fmt.Printf("%s\n", line.toActual())
	}

}