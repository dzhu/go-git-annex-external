package helper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// Required git-annex-initiated messages.
	cmdInitRemote   = "INITREMOTE"
	cmdPrepare      = "PREPARE"
	cmdTransfer     = "TRANSFER"
	cmdCheckPresent = "CHECKPRESENT"
	cmdRemove       = "REMOVE"

	// Optional git-annex-initated messages.
	cmdExtensions  = "EXTENSIONS"
	cmdListConfigs = "LISTCONFIGS"

	dirStore    = "STORE"
	dirRetrieve = "RETRIEVE"
)

var argCounts = map[string]int{
	cmdInitRemote:   0,
	cmdPrepare:      0,
	cmdTransfer:     3,
	cmdCheckPresent: 1,
	cmdRemove:       1,
	cmdExtensions:   1,
	cmdListConfigs:  0,
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
	GetConfig(name string) string
	SetConfig(name, value string)
	Progress(bytes int)
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
	Run()
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

func (r *remoteRunner) init() {
	if err := r.remote.Init(r); err != nil {
		r.sendFailure(cmdInitRemote, err)
		return
	}
	r.sendSuccess(cmdInitRemote)
}

func (r *remoteRunner) prepare() {
	if err := r.remote.Prepare(r); err != nil {
		r.sendFailure(cmdPrepare, err)
		return
	}
	r.sendSuccess(cmdPrepare)
}

func (r *remoteRunner) transfer(dir, key, file string) {
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
		r.sendFailure(cmdTransfer, dir, key, err)
		return
	}
	r.sendSuccess(cmdTransfer, dir, key)
}

func (r *remoteRunner) present(key string) {
	switch present, err := r.remote.Present(r, key); {
	case present:
		r.sendSuccess(cmdCheckPresent, key)
	case err != nil:
		r.sendUnknown(cmdCheckPresent, key, err)
	default:
		r.sendFailure(cmdCheckPresent, key)
	}
}

func (r *remoteRunner) remove(key string) {
	if err := r.remote.Remove(r, key); err != nil {
		r.sendFailure(cmdRemove, key, err)
		return
	}
	r.sendSuccess(cmdRemove, key)
}

func (r *remoteRunner) procLine(line string) {
	cmdAndArgs := strings.SplitN(line, " ", 2)
	cmd := cmdAndArgs[0]
	argCount, ok := argCounts[cmd]
	if !ok {
		r.unsupported()
		return
	}
	argsStr := ""
	if len(cmdAndArgs) > 1 {
		argsStr = cmdAndArgs[1]
	}
	args := strings.SplitN(argsStr, " ", argCount)
	switch cmd {
	case cmdInitRemote:
		r.init()
	case cmdPrepare:
		r.prepare()
	case cmdTransfer:
		r.transfer(args[0], args[1], args[2])
	case cmdCheckPresent:
		r.present(args[0])
	case cmdRemove:
		r.remove(args[0])
	case cmdExtensions:
		r.extensions(strings.Split(args[0], " "))
	case cmdListConfigs:
		r.listConfigs()
	}
}

func (r *remoteRunner) Run() {
	r.sendLine("VERSION 1")
	for line := r.getLine(); line != ""; line = r.getLine() {
		r.procLine(line)
	}
}

func Run(r RemoteV1) {
	Log("================ starting")
	NewRemote(os.Stdin, os.Stdout, r).Run()
}
