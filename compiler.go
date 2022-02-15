// lexer.go
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

func assemble(asm ASM, source, executable string) (err error) {

	var srcFile *os.File
	var e error

	if source == "" {
		srcFile, e = ioutil.TempFile("", "")
		if e != nil {
			err = fmt.Errorf("Creating temporary srcFile failed - %w", e)
			return
		}
		defer os.Remove(srcFile.Name())
	} else {
		srcFile, e = os.Create(source)
		if e != nil {
			err = fmt.Errorf("Creating srcFile failed - %w", e)
			return
		}
	}

	objectFile, e := ioutil.TempFile("", "")
	if e != nil {
		err = fmt.Errorf("Creating temporary objectFile failed - %w", e)
		return
	}
	objectFile.Close()

	// Write assembly into tmp source file
	defer os.Remove(objectFile.Name())

	for _, v := range asm.header {
		fmt.Fprintf(srcFile, "%v\n", v)
	}
	for _, v := range asm.constants {
		fmt.Fprintf(srcFile, "%-12v%-10v%-15v\n", v[0], "equ", v[1])
	}
	for _, v := range asm.variables {
		fmt.Fprintf(srcFile, "%-12v%-10v%-15v\n", v[0], v[1], v[2])
	}
	for _, v := range asm.program {
		fmt.Fprintf(srcFile, "%v%-10v%-10v\n", v[0], v[1], v[2])
	}
	srcFile.Close()

	// Find yasm
	yasm, e := exec.LookPath("yasm")
	if e != nil {
		err = fmt.Errorf("'yasm' not found. Please install - %w", e)
		return
	}
	// Assemble
	yasmCmd := &exec.Cmd{
		Path:   yasm,
		Args:   []string{yasm, "-Worphan-labels", "-g", "dwarf2", "-f", "elf64", srcFile.Name(), "-o", objectFile.Name()},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if e := yasmCmd.Run(); e != nil {
		err = fmt.Errorf("Error while assembling the source code - %w", e)
		return
	}

	// Find ld
	ld, e := exec.LookPath("ld")
	if e != nil {
		err = fmt.Errorf("'ld' not found. Please install - %w", e)
		return
	}
	// Link
	ldCmd := &exec.Cmd{
		Path:   ld,
		Args:   []string{ld, "-dynamic-linker", "/lib64/ld-linux-x86-64.so.2", "-o", executable, objectFile.Name(), "-lc"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if e := ldCmd.Run(); e != nil {
		err = fmt.Errorf("Error while linking object file - %w", e)
		return
	}
	return
}

func main() {
	var program []byte = []byte(`

//v = (10 + 5 + 3 + 2) * -1 * 3
// I can now write comments :)
//b = (2 == 2) && !!!false

j = true && false
for i = 0; i < 5; i = i+1 {
	a = i
}

`)

	tokenChan := make(chan Token, 1)
	lexerErr := make(chan error, 1)
	go tokenize(program, tokenChan, lexerErr)

	ast, parseErr := parse(tokenChan)

	// check error channel on incoming errors
	// As we lex and parse simultaneously, there is most likely a parser error as well. But that should be ignored
	// as long as we have token errors before!
	select {
	case e := <-lexerErr:
		fmt.Println(e)
		os.Exit(1)
	default:
	}

	if parseErr != nil {
		fmt.Println(parseErr)
		os.Exit(1)
	}

	ast, semanticErr := semanticAnalysis(ast)
	if semanticErr != nil {
		fmt.Println(semanticErr)
		os.Exit(1)
	}

	asm := ast.generateCode()

	if asmErr := assemble(asm, "source.asm", "executable"); asmErr != nil {
		fmt.Println(asmErr)
		os.Exit(1)
	}
}
