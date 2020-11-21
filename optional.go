package helper

import "strings"

type hasExtensions interface {
	Extensions(a Annex, e []string) []string
}

func (r *remoteRunner) extensions(e []string) {
	h, ok := r.remote.(hasExtensions)
	if !ok {
		r.unsupported()
		return
	}
	es := h.Extensions(r, e)
	r.sendLine(cmdExtensions, strings.Join(es, " "))
}

type hasListConfigs interface {
	ListConfigs(a Annex) [][]string
}

func (r *remoteRunner) listConfigs() {
	h, ok := r.remote.(hasListConfigs)
	if !ok {
		r.unsupported()
		return
	}
	for _, c := range h.ListConfigs(r) {
		r.sendLine("CONFIG", c[0], c[1])
	}
	r.sendLine("CONFIGEND")
}

type hasGetCost interface {
	GetCost(a Annex) int
}

func (r *remoteRunner) getCost() {
	h, ok := r.remote.(hasGetCost)
	if !ok {
		r.unsupported()
		return
	}
	r.sendLine("COST", h.GetCost(r))
}
