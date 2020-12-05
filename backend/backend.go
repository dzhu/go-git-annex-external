// Package backend implements the git-annex external backend protocol. It can be used to
// create an external backend without detailed knowledge of the git-annex wire protocol.
//
// For basic functionality, define a type implementing the BackendV1 interface and pass an instance
// of it to the Run function. Optional messages in the protocol may be supported by having the type
// additionally implement the "Has*" interfaces.
//
// See https://git-annex.branchable.com/design/external_backend_protocol/ for further information
// regarding the underlying protocol and the semantics of its operations.
package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dzhu/go-git-annex-external/internal"
)

const (
	cmdGetVersion                = "GETVERSION"
	cmdCanVerify                 = "CANVERIFY"
	cmdIsStable                  = "ISSTABLE"
	cmdIsCryptographicallySecure = "ISCRYPTOGRAPHICALLYSECURE"
	cmdGenKey                    = "GENKEY"
	cmdVerifyKeyContent          = "VERIFYKEYCONTENT"
)

// BackendV1 is the core interface that external backend implementations must satisfy.
type BackendV1 interface {
	// IsStable indicates whether this backend will always generate the same key for a given file.
	IsStable(a Annex) bool
	// GenKey returns the key name (just, e.g., a hash and not the full key) for the content of the
	// given file and whether to include the size field in the full key.
	GenKey(a Annex, file string) (string, bool, error)
}

// HasVerifyKeyContent is the interface that a backend implementation must implement to indicate
// that it supports verifying keys against files.
type HasVerifyKeyContent interface {
	// VerifyKeyContent checks whether the the given key is valid for the content of the given file.
	VerifyKeyContent(a Annex, key, file string) bool
	// IsCryptographicallySecure indicates whether the verification done by this backend is
	// cryptographically secure.
	IsCryptographicallySecure(a Annex) bool
}

// Annex allows external backend implementations to send requests to git-annex.
type Annex interface {
	Progress(bytes int)
	Debug(message string)
	Debugf(fmt string, args ...interface{})
	Error(message string)
	Errorf(fmt string, args ...interface{})
}

type annexIO struct {
	io   internal.LineIO
	impl BackendV1
	name string
}

func (a *annexIO) send(cmd string, args ...interface{}) {
	a.io.Send(cmd, args...)
}

func (a *annexIO) getVersion() {
	a.send("VERSION", 1)
}

func (a *annexIO) sendYes(cmd string, args ...interface{}) {
	a.send(cmd+"-YES", args...)
}

func (a *annexIO) sendNo(cmd string, args ...interface{}) {
	a.send(cmd+"-NO", args...)
}

func (a *annexIO) sendSuccess(cmd string, args ...interface{}) {
	a.send(cmd+"-SUCCESS", args...)
}

func (a *annexIO) sendFailure(cmd string, args ...interface{}) {
	a.send(cmd+"-FAILURE", args...)
}

func (a *annexIO) canVerify() {
	if _, ok := a.impl.(HasVerifyKeyContent); !ok {
		a.sendNo(cmdCanVerify)
		return
	}
	a.sendYes(cmdCanVerify)
}

func (a *annexIO) isStable() {
	if !a.impl.IsStable(a) {
		a.sendNo(cmdIsStable)
		return
	}
	a.sendYes(cmdIsStable)
}

func (a *annexIO) isCryptographicallySecure() {
	h, ok := a.impl.(HasVerifyKeyContent)
	if !ok || !h.IsCryptographicallySecure(a) {
		a.sendNo(cmdIsCryptographicallySecure)
		return
	}
	a.sendYes(cmdIsCryptographicallySecure)
}

func (a *annexIO) genKey(file string) {
	name, useSize, err := a.impl.GenKey(a, file)
	if err != nil {
		a.sendFailure(cmdGenKey, err)
		return
	}

	var key string

	if useSize {
		stat, err := os.Stat(file)
		if err != nil {
			a.sendFailure(cmdGenKey, err)
			return
		}
		key = fmt.Sprintf("X%s-s%d--%s", a.name, stat.Size(), name)
	} else {
		key = fmt.Sprintf("X%s--%s", a.name, name)
	}

	a.sendSuccess(cmdGenKey, key)
}

func (a *annexIO) verifyKeyContent(key, file string) {
	name := strings.SplitAfterN(key, "--", 2)[1]
	h, _ := a.impl.(HasVerifyKeyContent)
	if !h.VerifyKeyContent(a, name, file) {
		a.sendFailure(cmdVerifyKeyContent)
		return
	}
	a.sendSuccess(cmdVerifyKeyContent)
}

func (a *annexIO) Progress(bytes int) {
	a.send("PROGRESS", strconv.Itoa(bytes))
}

func (a *annexIO) Debug(message string) {
	a.send("DEBUG", message)
}

func (a *annexIO) Debugf(format string, args ...interface{}) {
	a.Debug(fmt.Sprintf(format, args...))
}

func (a *annexIO) Error(message string) {
	a.send("ERROR", message)
}

func (a *annexIO) Errorf(format string, args ...interface{}) {
	a.Error(fmt.Sprintf(format, args...))
}

func getBackendName() string {
	const prefix = "git-annex-backend-X"
	prog := filepath.Base(os.Args[0])
	rest := strings.TrimPrefix(prog, prefix)
	if rest == prog {
		return ""
	}
	return rest
}

// Run executes an external backend as git-annex expects, reading from stdin and writing to stdout.
func Run(b BackendV1) {
	name := getBackendName()
	internal.Run(func(lines internal.LineIO) map[string]internal.CommandSpec {
		a := &annexIO{io: lines, impl: b, name: name}
		return map[string]internal.CommandSpec{
			cmdGetVersion:                internal.Response0(a.getVersion),
			cmdCanVerify:                 internal.Response0(a.canVerify),
			cmdIsStable:                  internal.Response0(a.isStable),
			cmdIsCryptographicallySecure: internal.Response0(a.isCryptographicallySecure),
			cmdGenKey:                    internal.Response1(a.genKey),
			cmdVerifyKeyContent:          internal.Response2(a.verifyKeyContent),
		}
	})
}
