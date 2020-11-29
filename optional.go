package helper

import (
	"strconv"
	"strings"
)

// HasExtensions is the interface that a remote implementation must implement to support the
// EXTENSIONS command.
type HasExtensions interface {
	Extensions(a Annex, e []string) []string
}

func extensions(a *annexIO, r RemoteV1, e []string) {
	h, ok := r.(HasExtensions)
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

func listConfigs(a *annexIO, r RemoteV1) {
	h, ok := r.(HasListConfigs)
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

func getCost(a *annexIO, r RemoteV1) {
	h, ok := r.(HasGetCost)
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

func getAvailability(a *annexIO, r RemoteV1) {
	h, ok := r.(HasGetAvailability)
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

func claimURL(a *annexIO, r RemoteV1, url string) {
	h, ok := r.(HasClaimURL)
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

func checkURL(a *annexIO, r RemoteV1, url string) {
	h, ok := r.(HasCheckURL)
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

func whereIs(a *annexIO, r RemoteV1, key string) {
	h, ok := r.(HasWhereIs)
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

func getInfo(a *annexIO, r RemoteV1) {
	h, ok := r.(HasGetInfo)
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
