package main

import (
	"flag"
	"fmt"
	"github.com/coopernurse/barrister-go"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var outdir string
	var defaultPkgName string
	var baseImport string
	var optionalToPtr bool
	var quiet bool
	var tostdout bool
	var fromstdin bool

	flag.StringVar(&outdir, "d", ".", "Base directory to write generated .go files to")
	flag.StringVar(&defaultPkgName, "p", "", "Package name to write to generated Go file")
	flag.StringVar(&baseImport, "b", "", "Base import path for imported namespaces")
	flag.BoolVar(&optionalToPtr, "n", false, "If true, optional IDL fields will be generated as Go pointers")
	flag.BoolVar(&quiet, "q", false, "Enable quiet mode (no output)")
	flag.BoolVar(&tostdout, "s", false, "Write .go file to STDOUT (implies -q)")
	flag.BoolVar(&fromstdin, "i", false, "Read IDL JSON from STDIN")
	flag.Parse()

	if !fromstdin && flag.NArg() != 1 {
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

	if defaultPkgName == "" {
		defaultPkgName = baseName
	}

	if !quiet {
		if fromstdin {
			fmt.Println("Loading IDL from STDIN")
		} else {
			fmt.Println("Loading IDL from:", jsonFile)
		}
	}

	idl, err := parseIdl(fromstdin, jsonFile)
	if err != nil {
		from := jsonFile
		if fromstdin {
			from = "STDIN"
		}
		fmt.Fprintf(os.Stderr, "Error loading IDL from %s: %s\n", from, err)
		os.Exit(1)
	}

	pkgNameToGoCode := idl.GenerateGo(defaultPkgName, baseImport, optionalToPtr)
	for pkg, code := range pkgNameToGoCode {
		writeCode(quiet, tostdout, outdir, pkg, code)
	}
}

func writeCode(quiet bool, tostdout bool, outdir string, pkg string, code []byte) {

	if tostdout {
		fmt.Println(string(code))
	} else {

		dir := filepath.Join(outdir, pkg)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating dir %s: %s\n", dir, err)
			os.Exit(1)
		}
		outfile := filepath.Join(dir, fmt.Sprintf("%s.go", pkg))

		if !quiet {
			fmt.Printf("Generating %s as package %s\n", outfile, pkg)
		}

		err = ioutil.WriteFile(outfile, code, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file %s: %s\n", outfile, err)
			os.Exit(1)
		}
	}
}

func parseIdl(fromstdin bool, jsonFile string) (*barrister.Idl, error) {
	if fromstdin {
		jsonData, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return barrister.ParseIdlJson(jsonData)
	}

	return barrister.ParseIdlJsonFile(jsonFile)
}
