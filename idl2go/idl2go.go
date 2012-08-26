package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"github.com/coopernurse/barrister-go"
)

func main() {
	var outfile string
	var pkgname string
	var optionalToPtr bool
	var quiet bool
	var tostdout bool

	flag.StringVar(&outfile, "o", "", "File name to write generated .go file to")
	flag.StringVar(&pkgname, "p", "", "Package name to write to generated Go file")
	flag.BoolVar(&optionalToPtr, "n", false, "If true, optional IDL fields will be generated as Go pointers")
	flag.BoolVar(&quiet, "q", false, "Enable quiet mode (no output)")
	flag.BoolVar(&tostdout, "s", false, "Write .go file to STDOUT (implies -q)")
	flag.Parse()
	
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: idl2go jsonfile\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if tostdout {
		quiet = true
	}

	jsonFile := flag.Arg(0)
	baseName := filepath.Base(jsonFile)
	pos := strings.LastIndex(baseName, ".")
	if pos > -1 {
		baseName = baseName[0:pos]
	}

	if outfile == "" {
		outfile = baseName + ".go"
	}
	if pkgname == "" {
		pkgname = baseName
	}

	if !quiet {
		fmt.Printf("Loading IDL from: %s\n", jsonFile)
	}
	
	idl, err := barrister.ParseIdlJsonFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading IDL from %s: %s\n", jsonFile, err)
		os.Exit(1)
	}

	if !quiet {
		fmt.Printf("Generating %s as package %s\n", outfile, pkgname)
	}

	b := idl.GenerateGo(pkgname, optionalToPtr)
	if tostdout {
		fmt.Println(string(b))
	} else {
		err = ioutil.WriteFile(outfile, b, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file %s: %s\n", outfile, err)
			os.Exit(1)
		}
	}
}