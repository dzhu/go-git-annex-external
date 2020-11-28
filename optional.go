package helper

import "strings"

type hasExtensions interface {
	Extensions(a Annex, e []string) []string
}

func extensions(a *annexIO, r RemoteV1, e []string) {
	h, ok := r.(hasExtensions)
	if !ok {
		a.unsupported()
		return
	}
	es := h.Extensions(a, e)
	a.io.Send(cmdExtensions, strings.Join(es, " "))
}

type hasListConfigs interface {
	ListConfigs(a Annex) [][]string
}

func listConfigs(a *annexIO, r RemoteV1) {
	h, ok := r.(hasListConfigs)
	if !ok {
		a.unsupported()
		return
	}
	for _, c := range h.ListConfigs(a) {
		a.io.Send("CONFIG", c[0], c[1])
	}
	a.io.Send("CONFIGEND")
}

type hasGetCost interface {
	GetCost(a Annex) int
}

func getCost(a *annexIO, r RemoteV1) {
	h, ok := r.(hasGetCost)
	if !ok {
		a.unsupported()
		return
	}
	a.io.Send("COST", h.GetCost(a))
}
