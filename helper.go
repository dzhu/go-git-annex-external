package helper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	cmdInitRemote   = "INITREMOTE"
	cmdPrepare      = "PREPARE"
	cmdTransfer     = "TRANSFER"
	cmdCheckPresent = "CHECKPRESENT"
	cmdRemove       = "REMOVE"

	cmdGetConfig = "GETCONFIG"

	dirStore    = "STORE"
	dirRetrieve = "RETRIEVE"
)

var argCounts = map[string]int{
	cmdInitRemote:   0,
	cmdPrepare:      0,
	cmdTransfer:     3,
	cmdCheckPresent: 1,
	cmdRemove:       1,
}

var logger io.WriteCloser

func init() {
	var err error
	logger, err = os.OpenFile("/tmp/remote.log", os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
}

func Log(format string, args ...interface{}) {
	fmt.Fprintf(logger, format+"\n", args...)
}

type Annex interface {
	GetConfig(name string) (string, error)
}

type RemoteV1 interface {
	Init(a Annex) error
	Prepare(a Annex) error
	Store(a Annex, key, file string) error
	Retrieve(a Annex, key, file string) error
	Present(a Annex, key string) (bool, error)
	Remove(a Annex, key string) error
}

type Runner interface {
	Run() error
}

type remoteRunner struct {
	input   io.Reader
	output  io.Writer
	scanner *bufio.Scanner
	remote  RemoteV1
}

func NewRemote(input io.Reader, output io.Writer, r RemoteV1) Runner {
	return &remoteRunner{
		input:   input,
		output:  output,
		scanner: bufio.NewScanner(input),
		remote:  r,
	}
}

func (r *remoteRunner) getLine() (string, error) {
	switch {
	case !r.scanner.Scan():
		return "", nil
	case r.scanner.Err() != nil:
		return "", r.scanner.Err()
	default:
		return r.scanner.Text(), nil
	}
}

func (r *remoteRunner) sendLine(cmd string, args ...interface{}) error {
	s := []string{cmd}
	for _, arg := range args {
		s = append(s, fmt.Sprintf("%s", arg))
	}
	_, err := r.output.Write([]byte(strings.Join(s, " ") + "\n"))
	return err
}

func (r *remoteRunner) sendSuccess(cmd string, args ...interface{}) error {
	return r.sendLine(cmd+"-SUCCESS", args...)
}

func (r *remoteRunner) sendFailure(cmd string, args ...interface{}) error {
	return r.sendLine(cmd+"-FAILURE", args...)
}

func (r *remoteRunner) sendUnknown(cmd string, args ...interface{}) error {
	return r.sendLine(cmd+"-UNKNOWN", args...)
}

func (r *remoteRunner) GetConfig(name string) (string, error) {
	if err := r.sendLine(cmdGetConfig, name); err != nil {
		return "", err
	}
	resp, err := r.getLine()
	if err != nil {
		return "", err
	}
	Log("getconfig %q -> %q", name, resp)
	return strings.SplitN(resp, " ", 2)[1], nil
}

func (r *remoteRunner) init() error {
	if err := r.remote.Init(r); err != nil {
		return r.sendFailure(cmdInitRemote, err)
	}
	return r.sendSuccess(cmdInitRemote)
}

func (r *remoteRunner) prepare() error {
	if err := r.remote.Prepare(r); err != nil {
		return r.sendFailure(cmdPrepare, err)
	}
	return r.sendSuccess(cmdPrepare)
}

func (r *remoteRunner) transfer(dir, key, file string) error {
	var proc func(Annex, string, string) error
	switch dir {
	case dirRetrieve:
		proc = r.remote.Retrieve
	case dirStore:
		proc = r.remote.Store
	default:
		panic("unknown transfer direction " + dir)
	}
	if err := proc(r, key, file); err != nil {
		return r.sendFailure(cmdTransfer, dir, key, err)
	}
	return r.sendSuccess(cmdTransfer, dir, key)
}

func (r *remoteRunner) present(key string) error {
	switch present, err := r.remote.Present(r, key); {
	case present:
		return r.sendSuccess(cmdCheckPresent, key)
	case err != nil:
		return r.sendUnknown(cmdCheckPresent, key, err)
	default:
		return r.sendFailure(cmdCheckPresent, key)
	}
}

func (r *remoteRunner) remove(key string) error {
	if err := r.remote.Remove(r, key); err != nil {
		return r.sendFailure(cmdRemove, key, err)
	}
	return r.sendSuccess(cmdRemove, key)
}

func (r *remoteRunner) procLine(line string) error {
	Log("got line %q", line)
	cmdAndArgs := strings.SplitN(line, " ", 2)
	cmd := cmdAndArgs[0]
	argCount, ok := argCounts[cmd]
	if !ok {
		return r.sendLine("UNSUPPORTED-REQUEST")
	}
	argsStr := ""
	if len(cmdAndArgs) > 1 {
		argsStr = cmdAndArgs[1]
	}
	args := strings.SplitN(argsStr, " ", argCount)
	switch cmd {
	case cmdInitRemote:
		return r.init()
	case cmdPrepare:
		return r.prepare()
	case cmdTransfer:
		return r.transfer(args[0], args[1], args[2])
	case cmdCheckPresent:
		return r.present(args[0])
	case cmdRemove:
		return r.remove(args[0])
	}
	return nil
}

func (r *remoteRunner) Run() error {
	if err := r.sendLine("VERSION 1"); err != nil {
		return err
	}
	for {

		switch line, err := r.getLine(); {
		case err != nil:
			return err
		case line == "":
			return nil
		default:
			if err := r.procLine(line); err != nil {
				Log("proc err: %s", err)
				return err
			}
		}
	}
}

func Run(r RemoteV1) error {
	helper.Log("================ start")
	r := helper.NewRemote(os.Stdin, os.Stdout, r)
	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stdout, "failed: %s", err)
		os.Exit(1)
	}
}
