package tun

import (
	"net/netip"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/contrab/freelru"
	"github.com/sagernet/sing/contrab/maphash"
)

type DirectRouteSession struct {
	// IPVersion uint8
	// Network     uint8
	Source      netip.Addr
	Destination netip.Addr
}

type RouteMapping struct {
	status freelru.Cache[DirectRouteSession, DirectRouteAction]
}

func NewRouteMapping(timeout time.Duration) *RouteMapping {
	status := common.Must1(freelru.NewSharded[DirectRouteSession, DirectRouteAction](1024, maphash.NewHasher[DirectRouteSession]().Hash32))
	status.SetHealthCheck(func(session DirectRouteSession, action DirectRouteAction) bool {
		return !action.Timeout()
	})
	status.SetOnEvict(func(session DirectRouteSession, action DirectRouteAction) {
		action.Close()
	})
	return &RouteMapping{status}
}

func (m *RouteMapping) Lookup(session DirectRouteSession, constructor func() DirectRouteAction) DirectRouteAction {
	var created DirectRouteAction
	action, updated, ok := m.status.GetAndRefreshOrAdd(session, func() (DirectRouteAction, bool) {
		created = constructor()
		return created, created != nil && !created.Timeout()
	})
	if !ok {
		return created
	}
	if updated && action.Timeout() {
		action = constructor()
		m.status.Add(session, action)
	}
	return action
}
