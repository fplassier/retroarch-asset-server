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
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	retroarchHost string = "http://buildbot.libretro.com/assets/"
	defaultListen string = ":5164"
)

func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		director(req)
		req.Host = target.Host
	}
	return proxy
}

type inMemoryFile struct {
	*strings.Reader
	name string
}

func (f inMemoryFile) Close() error {
	return nil
}

func (f inMemoryFile) Readdir(count int) ([]fs.FileInfo, error) {
	return []fs.FileInfo{}, nil
}

func (f inMemoryFile) Name() string {
	return f.name
}

func (f inMemoryFile) Mode() fs.FileMode {
	return 0444
}

func (f inMemoryFile) ModTime() time.Time {
	return time.Now()
}

func (f inMemoryFile) IsDir() bool {
	return false
}

func (f inMemoryFile) Sys() any {
	return nil
}

func (f inMemoryFile) Stat() (fs.FileInfo, error) {
	return f, nil
}

type fileSystem struct {
	Indexed bool
	SubDirs bool
	Root    string
	Source  http.Dir
}

func (filesystem *fileSystem) Open(name string) (http.File, error) {
	name = name[len(filesystem.Root)-1:]
	if filesystem.Indexed {
		if filesystem.SubDirs {
			if name == "/.index-dirs" {
				root, err := filesystem.Source.Open(".")
				if err != nil {
					return nil, err
				}
				files, err := root.Readdir(0)
				if err != nil {
					return nil, err
				}
				result := strings.Builder{}
				for _, info := range files {
					if info.Mode().Type() == fs.ModeSymlink {
						info, err = os.Stat(path.Join(string(filesystem.Source), info.Name()))
						if err != nil {
							return nil, err
						}
					}
					if info.IsDir() {
						fmt.Fprintln(&result, info.Name())
					}
				}
				return inMemoryFile{strings.NewReader(result.String()), ".index-dirs"}, nil
			}
		}
		dir, base := path.Split(name)
		if base == ".index" {
			d, err := filesystem.Source.Open(dir)
			if err != nil {
				return nil, err
			}
			files, err := d.Readdir(0)
			if err != nil {
				return nil, err
			}
			result := strings.Builder{}
			for _, info := range files {
				if info.Mode().Type() == fs.ModeSymlink {
					info, err = os.Stat(path.Join(string(filesystem.Source), dir, info.Name()))
					if err != nil {
						return nil, err
					}
				}
				if info.Mode().IsRegular() {
					fmt.Fprintln(&result, info.Name())
				}
			}
			return inMemoryFile{strings.NewReader(result.String()), ".index"}, nil
		}
	}
	return filesystem.Source.Open(name)
}

type serveCommand struct {
	listen   string
	frontend string
	system   string
	rom      string
	cli      *flag.FlagSet
}

func newServeCommand() *serveCommand {
	result := &serveCommand{}
	result.listen = defaultListen
	result.cli = flag.NewFlagSet(result.Name(), flag.ExitOnError)
	result.cli.Func("listen", "Server listening address (default: "+defaultListen+")", func(s string) error {
		endPoint, err := net.ResolveTCPAddr("tcp", s)
		if err == nil {
			result.listen = endPoint.String()
		}
		return err
	})
	result.cli.StringVar(&result.frontend, "frontend", "", "path of the directory where frontend is stored (optional)")
	result.cli.StringVar(&result.system, "system", "", "path of the directory where systems are stored (optional)")
	result.cli.StringVar(&result.rom, "rom", "", "path of the directory where ROMs are stored (optional)")
	return result
}

func newServer(listen, frontend, system, rom string) *http.Server {
	handler := http.NewServeMux()
	proxyURL, _ := url.Parse(retroarchHost)
	if frontend == "" {
		handler.Handle("/frontend/", newReverseProxy(proxyURL))
	} else {
		handler.Handle("/frontend/", http.FileServer(&fileSystem{
			Indexed: false,
			SubDirs: false,
			Root:    "/frontend/",
			Source:  http.Dir(frontend),
		}))
	}
	if system == "" {
		handler.Handle("/system/", newReverseProxy(proxyURL))
	} else {
		handler.Handle("/system/", http.FileServer(&fileSystem{
			Indexed: true,
			SubDirs: false,
			Root:    "/system/",
			Source:  http.Dir(system),
		}))
	}
	if rom == "" {
		handler.Handle("/cores/", newReverseProxy(proxyURL))
	} else {
		handler.Handle("/cores/", http.FileServer(&fileSystem{
			Indexed: true,
			SubDirs: true,
			Root:    "/cores/",
			Source:  http.Dir(rom),
		}))
	}
	return &http.Server{Addr: listen, Handler: handler}
}

func (cmd *serveCommand) Name() string {
	return "serve"
}

func (cmd *serveCommand) Desc() string {
	return "Start the server (default command)."
}

func (cmd *serveCommand) PrintUsage() {
	cmd.cli.Usage()
}

func (cmd *serveCommand) Run(args []string) error {
	cmd.cli.Parse(args)
	if cmd.cli.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "Unknown argument", cmd.cli.Arg(0))
		cmd.cli.SetOutput(os.Stderr)
		cmd.cli.Usage()
		os.Exit(1)
	}
	server := newServer(cmd.listen, cmd.frontend, cmd.system, cmd.rom)
	fmt.Println("Listening on", cmd.listen)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
