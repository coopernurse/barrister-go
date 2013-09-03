package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/coopernurse/barrister-go"
	"io"
	"os"
	"strings"
)

type RpcCaller interface {
	Call(client barrister.Client) []string
}

type ParsedFile struct {
	calls []RpcCaller
}

type Batch struct {
	lines []ConformLine
}

func (b *Batch) Call(client barrister.Client) []string {
	batch := []barrister.JsonRpcRequest{}
	for _, line := range b.lines {
		batch = append(batch, barrister.JsonRpcRequest{Id: line.rpcid, Method: line.Method(), Params: line.Params()})
	}

	var result []string

	batchResp := client.CallBatch(batch)
	for _, resp := range batchResp {
		for _, line := range b.lines {
			if resp.Id == line.rpcid {
				line.HandleResponse(resp.Result, resp.Error)
				result = append(result, line.String())
				break
			}
		}
	}
	return result
}

type ConformLine struct {
	iface      string
	function   string
	params     string
	exp_status string
	exp_result string
	act_status string
	act_result string
	rpcid      string
}

func (line *ConformLine) String() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", line.iface, line.function, line.params, line.act_status, line.act_result)
}

func (line *ConformLine) Method() string {
	return fmt.Sprintf("%s.%s", line.iface, line.function)
}

func (line *ConformLine) Params() []interface{} {
	var params []interface{}
	err := json.Unmarshal([]byte(line.params), &params)
	if err != nil {
		panic(err)
	}
	return params
}

func (line *ConformLine) HandleResponse(result interface{}, err error) {
	rpcerr, ok := err.(*barrister.JsonRpcError)
	if ok && rpcerr != nil {
		line.act_result = fmt.Sprintf("%d", rpcerr.Code)
		line.act_status = "rpcerr"

	} else {
		line.act_status = "ok"

		body, err := json.Marshal(result)
		if err != nil {
			panic(err)
		}
		b, err := barrister.EncodeASCII(body)
		if err != nil {
			panic(err)
		}
		line.act_result = b.String()
	}
}

func (line *ConformLine) Call(client barrister.Client) []string {

	params := line.Params()
	res, err := client.Call(line.Method(), params...)
	line.HandleResponse(res, err)

	return []string{line.String()}
}

func parseLine(s string) ConformLine {
	st := strings.TrimSpace(s)
	if len(st) > 0 && string(st[0]) != "#" {
		cols := strings.Split(s, "|")
		if len(cols) == 5 {
			return ConformLine{cols[0], cols[1], cols[2], cols[3], cols[4], "???", "???", randHex(16)}
		}
	}

	return ConformLine{}
}

func parseFile(fname string) (ParsedFile, error) {
	parsed := ParsedFile{}
	var batch *Batch = nil

	file, err := os.Open(fname)
	if err != nil {
		return parsed, err
	}

	r := bufio.NewReader(file)
	for {
		b, _, err := r.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return parsed, err
		}

		line := strings.TrimSpace(string(b))

		if line == "start_batch" {
			batch = &Batch{}
		} else if line == "end_batch" {
			parsed.calls = append(parsed.calls, batch)
			batch = nil
		} else if line != "" && line[0] != '#' {
			pLine := parseLine(line)
			if batch == nil {
				parsed.calls = append(parsed.calls, &pLine)
			} else {
				batch.lines = append(batch.lines, pLine)
			}
		}
	}
	file.Close()

	return parsed, nil
}

func randHex(bytes int) string {
	buf := make([]byte, bytes)
	io.ReadFull(rand.Reader, buf)
	return fmt.Sprintf("%x", buf)
}

func main() {
	flag.Parse()
	infile := flag.Arg(0)
	outfile := flag.Arg(1)

	parsed, err := parseFile(infile)
	if err != nil {
		panic(err)
	}

	client := barrister.NewRemoteClient(&barrister.HttpTransport{Url: "http://localhost:9233"}, true)

	fo, err := os.Create(outfile)
	if err != nil {
		panic(err)
	}
	defer fo.Close()
	w := bufio.NewWriter(fo)

	for _, c := range parsed.calls {
		for _, line := range c.Call(client) {
			w.WriteString(fmt.Sprintf("%s\n", line))
		}
	}
	w.Flush()
}
