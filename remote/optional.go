package remote

import (
	"strconv"
	"strings"
)

// HasExtensions is the interface that a remote implementation must implement to support the
// EXTENSIONS command.
type HasExtensions interface {
	Extensions(a Annex, e []string) []string
}

func (a *annexIO) extensions(e []string) {
	h, ok := a.impl.(HasExtensions)
	if !ok {
		a.unsupported()
		return
	}
	es := h.Extensions(a, e)
	a.send(cmdExtensions, strings.Join(es, " "))
}

// ConfigSetting is one configuration setting that can be set for this remote. It is used for the
// LISTCONFIGS command.
type ConfigSetting struct {
	Name, Description string
}

// HasListConfigs is the interface that a remote implementation must implement to support the
// LISTCONFIGS command.
type HasListConfigs interface {
	ListConfigs(a Annex) []ConfigSetting
}

func (a *annexIO) listConfigs() {
	h, ok := a.impl.(HasListConfigs)
	if !ok {
		a.unsupported()
		return
	}
	for _, c := range h.ListConfigs(a) {
		a.send("CONFIG", c.Name, c.Description)
	}
	a.send("CONFIGEND")
}

// HasGetCost is the interface that a remote implementation must implement to support the GETCOST
// command.
type HasGetCost interface {
	GetCost(a Annex) int
}

func (a *annexIO) getCost() {
	h, ok := a.impl.(HasGetCost)
	if !ok {
		a.unsupported()
		return
	}
	a.send("COST", h.GetCost(a))
}

// HasGetAvailability is the interface that a remote implementation must implement to support the
// GETAVAILABILITY command.
type HasGetAvailability interface {
	GetAvailability(a Annex) string
}

func (a *annexIO) getAvailability() {
	h, ok := a.impl.(HasGetAvailability)
	if !ok {
		a.unsupported()
		return
	}
	a.send("AVAILABILITY", h.GetAvailability(a))
}

// HasClaimURL is the interface that a remote implementation must implement to support the CLAIMURL
// command.
type HasClaimURL interface {
	ClaimURL(a Annex, url string) bool
}

func (a *annexIO) claimURL(url string) {
	h, ok := a.impl.(HasClaimURL)
	if !ok {
		a.unsupported()
		return
	}
	if !h.ClaimURL(a, url) {
		a.sendFailure(cmdClaimURL)
		return
	}
	a.sendSuccess(cmdClaimURL)
}

// URLInfo contains information about one URL for use with the CHECKURL command.
type URLInfo struct {
	URL      string
	Size     int
	Filename string
}

// HasCheckURL is the interface that a remote implementation must implement to support the CHECKURL
// command. If CheckURL returns a slice containing one element with an empty URL field, that
// translates into a CHECKURL-CONTENTS response; otherwise, CHECKURL-MULTI is used.
type HasCheckURL interface {
	CheckURL(a Annex, url string) ([]URLInfo, error)
}

func (a *annexIO) checkURL(url string) {
	h, ok := a.impl.(HasCheckURL)
	if !ok {
		a.unsupported()
		return
	}
	urls, err := h.CheckURL(a, url)
	if err != nil {
		a.sendFailure(cmdCheckURL, err)
		return
	}

	szStr := func(sz int) string {
		if sz < 0 {
			return "UNKNOWN"
		}
		return strconv.Itoa(sz)
	}

	for _, u := range urls {
		if strings.ContainsAny(u.URL, " \n") {
			a.sendFailure(cmdCheckURL, "remote implementation returned a URL containing a space")
			return
		}
		if strings.ContainsAny(u.Filename, " \n") {
			a.sendFailure(cmdCheckURL, "remote implementation returned a filename containing a space")
			return
		}
	}

	if len(urls) == 1 && urls[0].Filename == "" {
		a.send(cmdCheckURL+"-CONTENTS", szStr(urls[0].Size), urls[0].Filename)
		return
	}
	var args []interface{}
	for _, u := range urls {
		args = append(args, u.URL, szStr(u.Size), u.Filename)
	}
	a.send(cmdCheckURL+"-MULTI", args...)
}

// HasWhereIs is the interface that a remote implementation must implement to support the WHEREIS
// command.
type HasWhereIs interface {
	WhereIs(a Annex, url string) string
}

func (a *annexIO) whereIs(key string) {
	h, ok := a.impl.(HasWhereIs)
	if !ok {
		a.unsupported()
		return
	}
	w := h.WhereIs(a, key)
	if w == "" {
		a.sendFailure(cmdWhereIs)
		return
	}
	a.sendSuccess(cmdWhereIs, w)
}

// InfoField is one field to include in the output of `git annex info`. It is used for the GETINFO
// command.
type InfoField struct {
	Name, Value string
}

// HasGetInfo is the interface that a remote implementation must implement to support the GETINFO
// command.
type HasGetInfo interface {
	GetInfo(a Annex) []InfoField
}

func (a *annexIO) getInfo() {
	h, ok := a.impl.(HasGetInfo)
	if !ok {
		a.unsupported()
		return
	}
	for _, f := range h.GetInfo(a) {
		a.send("INFOFIELD", f.Name)
		a.send("INFOVALUE", f.Value)
	}
	a.send("INFOEND")
}

// HasExport is the interface that a remote implementation must implement to support the simple
// export interface.
type HasExport interface {
	// Store associates the content of the given file with the given key in the remote.
	StoreExport(a Annex, name, key, file string) error
	// Retrieve places the content of the given key into the given file.
	RetrieveExport(a Annex, name, key, file string) error
	// Present checks whether the remote contains the data for the given key.
	PresentExport(a Annex, name, key string) (bool, error)
	// Remove removes the content of the given key from the remote.
	RemoveExport(a Annex, name, key string) error
}

func (a *annexIO) exportSupported() {
	if _, ok := a.impl.(HasExport); !ok {
		a.sendFailure(cmdExportSupported)
		return
	}
	a.sendSuccess(cmdExportSupported)
}

func (a *annexIO) export(name string) {
	a.exportName = name
}

func (a *annexIO) presentExport(key string) {
	h, ok := a.impl.(HasExport)
	if !ok {
		a.unsupported()
		return
	}
	switch present, err := h.PresentExport(a, a.exportName, key); {
	case present:
		a.sendSuccess(cmdCheckPresent, key)
	case err != nil:
		a.sendUnknown(cmdCheckPresent, key, err)
	default:
		a.sendFailure(cmdCheckPresent, key)
	}
}

func (a *annexIO) transferExport(dir, key, file string) {
	h, ok := a.impl.(HasExport)
	if !ok {
		a.unsupported()
		return
	}
	var proc func(Annex, string, string, string) error
	switch dir {
	case dirRetrieve:
		proc = h.RetrieveExport
	case dirStore:
		proc = h.StoreExport
	default:
		panic("unknown transfer direction " + dir)
	}
	if err := proc(a, a.exportName, key, file); err != nil {
		a.sendFailure(cmdTransfer, dir, key, err)
		return
	}
	a.sendSuccess(cmdTransfer, dir, key)
}

func (a *annexIO) removeExport(key string) {
	h, ok := a.impl.(HasExport)
	if !ok {
		a.unsupported()
		return
	}
	if err := h.RemoveExport(a, a.exportName, key); err != nil {
		a.sendFailure(cmdRemove, key, err)
		return
	}
	a.sendSuccess(cmdRemove, key)
}

// HasRemoveExportDirectory is the interface that a remote implementation must implement to support
// the REMOVEEXPORTDIRECTORY command.
type HasRemoveExportDirectory interface {
	RemoveExportDirectory(a Annex, directory string) error
}

func (a *annexIO) removeExportDirectory(directory string) {
	h, ok := a.impl.(HasRemoveExportDirectory)
	if !ok {
		a.unsupported()
		return
	}
	if err := h.RemoveExportDirectory(a, directory); err != nil {
		a.sendFailure(cmdRemoveExportDirectory)
		return
	}
	a.sendSuccess(cmdRemoveExportDirectory)
}

// HasRenameExport is the interface that a remote implementation must implement to support the
// RENAMEEXPORT command.
type HasRenameExport interface {
	RenameExport(a Annex, name, key, newName string) error
}

func (a *annexIO) renameExport(name, key, newName string) {
	h, ok := a.impl.(HasRenameExport)
	if !ok {
		a.unsupported()
		return
	}
	if err := h.RenameExport(a, name, key, newName); err != nil {
		a.sendFailure(cmdRenameExport, key)
		return
	}
	a.sendSuccess(cmdRenameExport, key)
}
