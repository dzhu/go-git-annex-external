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

func (a *annexIO) startup() {
	a.send("VERSION 1")
}

func (a *annexIO) initialize() {
	if err := a.impl.Init(a); err != nil {
		a.sendFailure(cmdInitRemote, err)
		return
	}
	a.sendSuccess(cmdInitRemote)
}

func (a *annexIO) prepare() {
	if err := a.impl.Prepare(a); err != nil {
		a.sendFailure(cmdPrepare, err)
		return
	}
	a.sendSuccess(cmdPrepare)
}

func (a *annexIO) transfer(dir, key, file string) {
	var proc func(Annex, string, string) error
	switch dir {
	case dirRetrieve:
		proc = a.impl.Retrieve
	case dirStore:
		proc = a.impl.Store
	default:
		panic("unknown transfer direction " + dir)
	}
	if err := proc(a, key, file); err != nil {
		a.sendFailure(cmdTransfer, dir, key, err)
		return
	}
	a.sendSuccess(cmdTransfer, dir, key)
}

func (a *annexIO) present(key string) {
	switch present, err := a.impl.Present(a, key); {
	case present:
		a.sendSuccess(cmdCheckPresent, key)
	case err != nil:
		a.sendUnknown(cmdCheckPresent, key, err)
	default:
		a.sendFailure(cmdCheckPresent, key)
	}
}

func (a *annexIO) remove(key string) {
	if err := a.impl.Remove(a, key); err != nil {
		a.sendFailure(cmdRemove, key, err)
		return
	}
	a.sendSuccess(cmdRemove, key)
}

// Run executes an external special remote as git-annex expects, reading from stdin and writing to
// stdout.
func Run(r RemoteV1) {
	internal.Run(func(lines internal.LineIO) map[string]internal.CommandSpec {
		a := &annexIO{io: lines, impl: r}

		return map[string]internal.CommandSpec{
			internal.StartupCmd:      internal.Response0(a.startup),
			internal.UnsupportedCmd:  internal.Response0(a.unsupported),
			cmdInitRemote:            internal.Response0(a.initialize),
			cmdPrepare:               internal.Response0(a.prepare),
			cmdTransfer:              internal.Response3(a.transfer),
			cmdCheckPresent:          internal.Response1(a.present),
			cmdRemove:                internal.Response1(a.remove),
			cmdExtensions:            internal.ResponseSplit(a.extensions),
			cmdListConfigs:           internal.Response0(a.listConfigs),
			cmdGetCost:               internal.Response0(a.getCost),
			cmdGetAvailability:       internal.Response0(a.getAvailability),
			cmdClaimURL:              internal.Response1(a.claimURL),
			cmdCheckURL:              internal.Response1(a.checkURL),
			cmdWhereIs:               internal.Response1(a.whereIs),
			cmdGetInfo:               internal.Response0(a.getInfo),
			cmdExportSupported:       internal.Response0(a.exportSupported),
			cmdExport:                internal.Response1(a.export),
			cmdCheckPresentExport:    internal.Response1(a.presentExport),
			cmdTransferExport:        internal.Response3(a.transferExport),
			cmdRemoveExport:          internal.Response1(a.removeExport),
			cmdRemoveExportDirectory: internal.Response1(a.removeExportDirectory),
			cmdRenameExport:          internal.Response3(a.renameExport),
		}
	})
}
