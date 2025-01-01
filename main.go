// Copyright (c) 2024 Fabien Plassier
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const version string = "1.1"

func help(args []string) error {
	if len(args) == 0 {
		usage(os.Stdout, filepath.Base(os.Args[0]))
		os.Exit(0)
	} else {
		for _, cmd := range commands {
			if cmd.Name() == args[0] {
				fmt.Println(cmd.Desc())
				cmd.PrintUsage()
				os.Exit(0)
			}
		}
	}
	return fmt.Errorf("Unknown command %s", args[0])
}

type command interface {
	Name() string
	Desc() string
	PrintUsage()
	Run([]string) error
}

type versionCommand struct{}

func (cmd versionCommand) Name() string {
	return "version"
}

func (cmd versionCommand) Desc() string {
	return "Print the application version."
}

func (cmd versionCommand) PrintUsage() {}

func (cmd versionCommand) Run(args []string) error {
	fmt.Println(filepath.Base(os.Args[0]), "version", version)
	return nil
}

var commands []command = []command{versionCommand{}, newServeCommand()}

func usage(w io.Writer, name string) {
	fmt.Fprintf(w, "Usage: %s COMMAND [OPTIONS...]\nAvailable commands:\n", name)
	fmt.Fprintln(w, "  help: print this help or the provided command help")
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %s: %s\n", cmd.Name(), cmd.Desc())
	}
}

func main() {
	var defaultCommand command = nil
	var cmd command = nil
	var optIndex uint = 1
	if len(os.Args) < 2 {
		for _, c := range commands {
			if c.Name() == "serve" {
				defaultCommand = c
				break
			}
		}
	} else if os.Args[1] == "help" {
		err := help(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	} else {
		for _, c := range commands {
			if c.Name() == "serve" {
				defaultCommand = c
			}
			if c.Name() == os.Args[1] {
				cmd = c
				optIndex = 2
				break
			}
		}
	}
	if cmd != nil {
		err := cmd.Run(os.Args[optIndex:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else if defaultCommand != nil {
		err := defaultCommand.Run(os.Args[optIndex:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Unknown command", os.Args[1])
		usage(os.Stderr, filepath.Base(os.Args[0]))
		os.Exit(1)
	}
}
