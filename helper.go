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
	cmdGetCost     = "GETCOST"

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
	Progress(bytes int)
	DirHash(key string) string
	DirHashLower(key string) string
	SetConfig(setting, value string)
	GetConfig(setting string) string
	SetCreds(setting, user, password string)
	GetCreds(setting string) (string, string)
	GetUUID() string
	GetGitDir() string
	SetWanted(expression string)
	GetWanted() string
	SetState(setting, value string)
	GetState(setting string) string
	SetURLPresent(key, url string)
	SetURLMissing(key, url string)
	SetURIPresent(key, uri string)
	SetURIMissing(key, uri string)
	GetURLs(key, prefix string) []string
	Debug(message string)
	Debugf(fmt string, args ...interface{})
	Info(message string)
	Infof(fmt string, args ...interface{})
	Error(message string)
	Errorf(fmt string, args ...interface{})
}

type RemoteV1 interface {
	Init(a Annex) error
	Prepare(a Annex) error
	Store(a Annex, key, file string) error
	Retrieve(a Annex, key, file string) error
	Present(a Annex, key string) (bool, error)
	Remove(a Annex, key string) error
}

func initialize(a *annexIO, r RemoteV1) {
	if err := r.Init(a); err != nil {
		a.sendFailure(cmdInitRemote, err)
		return
	}
	a.sendSuccess(cmdInitRemote)
}

func prepare(a *annexIO, r RemoteV1) {
	if err := r.Prepare(a); err != nil {
		a.sendFailure(cmdPrepare, err)
		return
	}
	a.sendSuccess(cmdPrepare)
}

func transfer(a *annexIO, r RemoteV1, dir, key, file string) {
	var proc func(Annex, string, string) error
	switch dir {
	case dirRetrieve:
		proc = r.Retrieve
	case dirStore:
		proc = r.Store
	default:
		panic("unknown transfer direction " + dir)
	}
	if err := proc(a, key, file); err != nil {
		a.sendFailure(cmdTransfer, dir, key, err)
		return
	}
	a.sendSuccess(cmdTransfer, dir, key)
}

func present(a *annexIO, r RemoteV1, key string) {
	switch present, err := r.Present(a, key); {
	case present:
		a.sendSuccess(cmdCheckPresent, key)
	case err != nil:
		a.sendUnknown(cmdCheckPresent, key, err)
	default:
		a.sendFailure(cmdCheckPresent, key)
	}
}

func remove(a *annexIO, r RemoteV1, key string) {
	if err := r.Remove(a, key); err != nil {
		a.sendFailure(cmdRemove, key, err)
		return
	}
	a.sendSuccess(cmdRemove, key)
}

func procLine(a *annexIO, r RemoteV1, line string) {
	cmdAndArgs := strings.SplitN(line, " ", 2)
	cmd := cmdAndArgs[0]
	argCount, ok := argCounts[cmd]
	if !ok {
		a.unsupported()
		return
	}
	argsStr := ""
	if len(cmdAndArgs) > 1 {
		argsStr = cmdAndArgs[1]
	}
	args := strings.SplitN(argsStr, " ", argCount)
	switch cmd {
	case cmdInitRemote:
		initialize(a, r)
	case cmdPrepare:
		prepare(a, r)
	case cmdTransfer:
		transfer(a, r, args[0], args[1], args[2])
	case cmdCheckPresent:
		present(a, r, args[0])
	case cmdRemove:
		remove(a, r, args[0])
	case cmdExtensions:
		extensions(a, r, strings.Split(args[0], " "))
	case cmdListConfigs:
		listConfigs(a, r)
	case cmdGetCost:
		getCost(a, r)
	}
}

func Run(r RemoteV1) {
	lines := &rawLineIO{
		w: os.Stdout,
		s: bufio.NewScanner(os.Stdin),
	}
	a := &annexIO{io: lines}
	lines.Send("VERSION 1")
	defer func() {
		if p := recover(); p != nil {
			a.Error(fmt.Sprintf("failed: %s", p))
		}
	}()
	for line := lines.Recv(); line != ""; line = lines.Recv() {
		procLine(a, r, line)
	}
}
