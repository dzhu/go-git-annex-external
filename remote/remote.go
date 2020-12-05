// Package remote implements the git-annex external special remote protocol. It can be used to
// create an external special remote without detailed knowledge of the git-annex wire protocol. It
// supports the ASYNC and INFO protocol extensions.
//
// For basic functionality, define a type implementing the RemoteV1 interface and pass an instance
// of it to the Run function. Optional messages in the protocol may be supported by having the type
// additionally implement the "Has*" interfaces.
//
// See https://git-annex.branchable.com/design/external_special_remote_protocol/ for further
// information regarding the underlying protocol and the semantics of its operations.
package remote

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dzhu/go-git-annex-external/internal"
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

	// Export interface messages.
	cmdExportSupported       = "EXPORTSUPPORTED"
	cmdExport                = "EXPORT"
	cmdCheckPresentExport    = "CHECKPRESENTEXPORT"
	cmdTransferExport        = "TRANSFEREXPORT"
	cmdRemoveExport          = "REMOVEEXPORT"
	cmdRemoveExportDirectory = "REMOVEEXPORTDIRECTORY"
	cmdRenameExport          = "RENAMEEXPORT"

	dirStore    = "STORE"
	dirRetrieve = "RETRIEVE"
)

const (
	// ExtInfo is the keyword of the protocol extension for info messages.
	ExtInfo = "INFO"
	// ExtAsync is the keyword of the protocol extension for asynchronous jobs.
	ExtAsync = "ASYNC"
)

func response0(a *annexIO, r RemoteV1, f func(a *annexIO, r RemoteV1)) internal.CommandSpec {
	return internal.CommandSpec{0, func(args []string) { f(a, r) }}
}

func response1(a *annexIO, r RemoteV1, f func(a *annexIO, r RemoteV1, s string)) internal.CommandSpec {
	return internal.CommandSpec{1, func(args []string) { f(a, r, args[0]) }}
}

func response3(a *annexIO, r RemoteV1, f func(a *annexIO, r RemoteV1, s1, s2, s3 string)) internal.CommandSpec {
	return internal.CommandSpec{
		3, func(args []string) { f(a, r, args[0], args[1], args[2]) },
	}
}

func responseSplit(a *annexIO, r RemoteV1, f func(a *annexIO, r RemoteV1, s []string)) internal.CommandSpec {
	return internal.CommandSpec{
		1, func(args []string) { f(a, r, strings.Split(args[0], " ")) },
	}
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

func startup(a *annexIO, r RemoteV1) {
	a.send("VERSION 1")
}

func unsupported(a *annexIO, r RemoteV1) {
	a.unsupported()
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

// Run executes an external special remote as git-annex expects, reading from stdin and writing to
// stdout.
func Run(r RemoteV1) {
	internal.Run(func(lines internal.LineIO) map[string]internal.CommandSpec {
		a := &annexIO{io: lines}

		return map[string]internal.CommandSpec{
			internal.StartupCmd:      response0(a, r, startup),
			internal.UnsupportedCmd:  response0(a, r, unsupported),
			cmdInitRemote:            response0(a, r, initialize),
			cmdPrepare:               response0(a, r, prepare),
			cmdTransfer:              response3(a, r, transfer),
			cmdCheckPresent:          response1(a, r, present),
			cmdRemove:                response1(a, r, remove),
			cmdExtensions:            responseSplit(a, r, extensions),
			cmdListConfigs:           response0(a, r, listConfigs),
			cmdGetCost:               response0(a, r, getCost),
			cmdGetAvailability:       response0(a, r, getAvailability),
			cmdClaimURL:              response1(a, r, claimURL),
			cmdCheckURL:              response1(a, r, checkURL),
			cmdWhereIs:               response1(a, r, whereIs),
			cmdGetInfo:               response0(a, r, getInfo),
			cmdExportSupported:       response0(a, r, exportSupported),
			cmdExport:                response1(a, r, export),
			cmdCheckPresentExport:    response1(a, r, presentExport),
			cmdTransferExport:        response3(a, r, transferExport),
			cmdRemoveExport:          response1(a, r, removeExport),
			cmdRemoveExportDirectory: response1(a, r, removeExportDirectory),
			cmdRenameExport:          response3(a, r, renameExport),
		}
	})
}
