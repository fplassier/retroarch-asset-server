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
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName string = "retroarch-asset-server"
)

type windowsService struct {
	elog *eventlog.Log
}

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}
	argsHelper := newRegisterSvcCommand(false)
	err := argsHelper.cli.Parse(os.Args[1:])
	if err != nil {
		ws.elog.Error(1, fmt.Sprintf("Internal error: %s. You shoud unregister the service then register it again.", err.Error()))
		s <- svc.Status{State: svc.Stopped}
		return true, 1
	}
	err = argsHelper.cli.Parse(args[1:])
	if err != nil {
		ws.elog.Error(1, fmt.Sprintf("Invalid options: %s", err.Error()))
		s <- svc.Status{State: svc.Stopped}
		return true, 1
	}
	if argsHelper.listen == "" {
		argsHelper.listen = defaultListen
	}

	ws.elog.Info(1, fmt.Sprintf("Listening on %s", argsHelper.listen))
	ws.elog.Info(1, fmt.Sprintf("Frontend path: %s", argsHelper.frontend))
	ws.elog.Info(1, fmt.Sprintf("System path: %s", argsHelper.system))
	ws.elog.Info(1, fmt.Sprintf("ROM path: %s", argsHelper.rom))
	server := newServer(argsHelper.listen, argsHelper.frontend, argsHelper.system, argsHelper.rom)
	ctxt, cancel := context.WithCancel(context.Background())
	go func() {
		err := server.ListenAndServe()
		if err != nil && (err != http.ErrServerClosed) {
			ws.elog.Error(1, fmt.Sprintf("HTTP server error: %s", err.Error()))
		}
		cancel()
	}()

	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				server.Shutdown(context.Background())
				s <- svc.Status{State: svc.StopPending}
			default:
				ws.elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		case <-ctxt.Done():
			break loop
		}
	}
	s <- svc.Status{State: svc.StopPending}
	return false, 0
}

type registerSvcCommand struct {
	listen   string
	frontend string
	system   string
	rom      string
	cli      *flag.FlagSet
}

func newRegisterSvcCommand(exitOnArgError bool) *registerSvcCommand {
	result := &registerSvcCommand{}
	if exitOnArgError {
		result.cli = flag.NewFlagSet(result.Name(), flag.ExitOnError)
	} else {
		result.cli = flag.NewFlagSet(result.Name(), flag.ContinueOnError)
	}
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

func (cmd *registerSvcCommand) Name() string {
	return "register-svc"
}

func (cmd *registerSvcCommand) Desc() string {
	return "Register a Windows auto-starting service to launch the server."
}

func (cmd *registerSvcCommand) PrintUsage() {
	cmd.cli.Usage()
}

func (cmd *registerSvcCommand) Run(args []string) error {
	cmd.cli.Parse(args)
	if cmd.cli.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "Unknown argument", cmd.cli.Arg(0))
		cmd.cli.SetOutput(os.Stderr)
		cmd.cli.Usage()
		os.Exit(1)
	}

	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	if svc, err := manager.OpenService(serviceName); err == nil {
		svc.Close()
		return fmt.Errorf("Service %s already exists", serviceName)
	}
	exepath, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	if filepath.Ext(exepath) == "" {
		exepath += ".exe"
	}

	conf := mgr.Config{
		DisplayName: "Retroarch asset server",
		StartType:   mgr.StartAutomatic,
	}
	svcArgs := []string{}
	if len(cmd.listen) > 0 {
		svcArgs = append(svcArgs, "-listen", cmd.listen)
	}
	if len(cmd.frontend) > 0 {
		cmd.frontend, err = filepath.Abs(cmd.frontend)
		if err != nil {
			return err
		}
		svcArgs = append(svcArgs, "-frontend", cmd.frontend)
	}
	if len(cmd.system) > 0 {
		cmd.system, err = filepath.Abs(cmd.system)
		if err != nil {
			return err
		}
		svcArgs = append(svcArgs, "-system", cmd.system)
	}
	if len(cmd.rom) > 0 {
		cmd.rom, err = filepath.Abs(cmd.rom)
		if err != nil {
			return err
		}
		svcArgs = append(svcArgs, "-rom", cmd.rom)
	}
	service, err := manager.CreateService(serviceName, exepath, conf, svcArgs...)
	if err != nil {
		return err
	}
	defer service.Close()
	err = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		service.Delete()
		return err
	}
	err = service.Start()
	if err != nil {
		eventlog.Remove(serviceName)
		service.Delete()
		return err
	}
	return nil
}

type unregisterSvcCommand struct{}

func (cmd unregisterSvcCommand) Name() string {
	return "unregister-svc"
}

func (cmd unregisterSvcCommand) Desc() string {
	return "Unregister the Windows auto-starting service that launch the server."
}

func (cmd unregisterSvcCommand) PrintUsage() {}

func (cmd unregisterSvcCommand) Run(args []string) error {
	mgr, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer mgr.Disconnect()
	service, err := mgr.OpenService(serviceName)
	if err != nil {
		return err
	}
	defer service.Close()
	for {
		status, err := service.Query()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get the service state: %s. It will be deleted after restart.", err.Error())
			break
		}
		if status.State == svc.Stopped {
			break
		}
		_, err = service.Control(svc.Stop)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not stop the service: %s. It will be deleted after restart.", err.Error())
			break
		}
		time.Sleep(time.Second)
	}
	err = service.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(serviceName)
	if err != nil {
		return err
	}
	return nil
}

func registerExtraCommands() {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(255)
	}
	if isSvc {
		elog, err := eventlog.Open(serviceName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(255)
		}
		defer elog.Close()

		elog.Info(1, fmt.Sprintf("Starting service %s", serviceName))
		err = svc.Run(serviceName, &windowsService{elog})
		if err != nil {
			elog.Error(1, fmt.Sprintf("Service %s failed: %v", serviceName, err))
			os.Exit(255)
		}
		elog.Info(1, fmt.Sprintf("Service %s stopped", serviceName))
		os.Exit(0)
	} else {
		commands = append(commands, newRegisterSvcCommand(true), unregisterSvcCommand{})
	}
}
