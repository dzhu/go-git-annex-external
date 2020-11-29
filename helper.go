// Package helper implements the git-annex external special remote protocol. It can be used to
// create an external special remote without detailed knowledge of the git-annex wire protocol. It
// supports the ASYNC and INFO protocol extensions.
//
// For basic functionality, define a type implementing the RemoteV1 interface and pass an instance
// of it to the Run function. Optional messages in the protocol may be supported by having the type
// additionally implement the "Has*" interfaces.
//
// See https://git-annex.branchable.com/design/external_special_remote_protocol/ for further
// information regarding the underlying protocol and the semantics of its operations.
package helper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
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
	cmdExtensions      = "EXTENSIONS"
	cmdListConfigs     = "LISTCONFIGS"
	cmdGetCost         = "GETCOST"
	cmdGetAvailability = "GETAVAILABILITY"
	cmdClaimURL        = "CLAIMURL"
	cmdCheckURL        = "CHECKURL"
	cmdWhereIs         = "WHEREIS"
	cmdGetInfo         = "GETINFO"

	dirStore    = "STORE"
	dirRetrieve = "RETRIEVE"
)

var argCounts = map[string]int{
	cmdInitRemote:      0,
	cmdPrepare:         0,
	cmdTransfer:        3,
	cmdCheckPresent:    1,
	cmdRemove:          1,
	cmdExtensions:      1,
	cmdListConfigs:     0,
	cmdGetCost:         0,
	cmdGetAvailability: 0,
	cmdClaimURL:        1,
	cmdCheckURL:        1,
	cmdWhereIs:         1,
	cmdGetInfo:         0,
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

// Annex allows external special remote implementations to send requests to git-annex.
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

// RemoteV1 is the core interface that external special remote implementations must satisfy.
type RemoteV1 interface {
	// Init performs one-time setup tasks required to use the remote. It is not called every time
	// git-annex interacts with the remote, but it may be called multiple times when the remote is
	// enabled in different repositories or when a configuration value is changed.
	Init(a Annex) error
	// Prepare prepares the remote to be used. It is called once each time the remote is run, before
	// any other methods that involve manipulating data in the remote.
	Prepare(a Annex) error
	// Store associates the content of the given file with the given key in the remote.
	Store(a Annex, key, file string) error
	// Retrieve places the content of the given key into the given file.
	Retrieve(a Annex, key, file string) error
	// Present checks whether the remote contains the data for the given key.
	Present(a Annex, key string) (bool, error)
	// Remove removes the content of the given key from the remote.
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
	case cmdGetAvailability:
		getAvailability(a, r)
	case cmdClaimURL:
		claimURL(a, r, args[0])
	case cmdCheckURL:
		checkURL(a, r, args[0])
	case cmdWhereIs:
		whereIs(a, r, args[0])
	case cmdGetInfo:
		getInfo(a, r)
	}
}

func getJobNum(line string) int {
	split := strings.SplitN(line, " ", 3)
	if split[0] != "J" {
		return 0
	}
	jobNum, _ := strconv.Atoi(split[1])
	return jobNum
}

func runJob(lines lineIO, r RemoteV1) {
	a := &annexIO{lines}
	for line := lines.Recv(); line != ""; line = lines.Recv() {
		procLine(a, r, line)
	}
}

// RunWithStreams executes an external special remote with the provided input and output streams.
func RunWithStreams(input io.Reader, output io.Writer, r RemoteV1) {
	lines := &rawLineIO{
		w: output,
		s: bufio.NewScanner(input),
	}
	a := &annexIO{io: lines}
	lines.Send("VERSION 1")
	defer func() {
		if p := recover(); p != nil {
			a.Error(fmt.Sprintf("failed: %s", p))
		}
	}()

	outLines := make(chan string)

	go func() {
		for l := range outLines {
			Log("\x1b[32m[%8d] -> %s\x1b[m", os.Getpid(), l)
			lines.Send(l)
		}
	}()

	inChans := make(map[int]chan string)

	for line := lines.Recv(); line != ""; line = lines.Recv() {
		Log("\x1b[34m[%8d] <- %s\x1b[m", os.Getpid(), line)

		jobNum := getJobNum(line)
		ch, ok := inChans[jobNum]
		if !ok {
			ch = make(chan string)
			inChans[jobNum] = ch
			go runJob(&jobLineIO{ch, jobNum, outLines}, r)
		}
		ch <- line
	}
}

// Run executes an external special remote as git-annex expects, reading from stdin and writing to
// stdout.
func Run(r RemoteV1) {
	RunWithStreams(os.Stdin, os.Stdout, r)
}
