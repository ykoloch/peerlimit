package peerlimit

import (
	"context"
	"io"

	"github.com/hashicorp/memberlist"
)

// delegate bridges the store to memberlist: it ships this node's payload (the
// G-Counter counts plus lastSeen) as local state and merges the payloads peers
// send back. The remaining memberlist.Delegate methods are unused.
type delegate struct {
	store *store
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte(d.store.node)
}

func (d *delegate) NotifyMsg(msg []byte) {
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState returns this node's serialised payload (counts + lastSeen) for
// memberlist to push to a peer.
func (d *delegate) LocalState(_ bool) []byte {
	pl, err := d.store.snapshot()
	if err != nil {
		return nil
	}
	return pl
}

// MergeRemoteState folds a peer's payload into the local store. A malformed
// payload is dropped rather than corrupting local state.
func (d *delegate) MergeRemoteState(buf []byte, _ bool) {
	pl, err := unmarshalPayload(buf)
	if err != nil {
		return
	}
	d.store.merge(pl)
}

// startGossip configures and starts memberlist for s, then joins the seeds the
// Discoverer returns.
func startGossip(ctx context.Context, s *store, conf Config) (*memberlist.Memberlist, error) {
	mlConf := memberlist.DefaultLANConfig()
	if conf.LogOutput != nil {
		mlConf.LogOutput = conf.LogOutput
	} else {
		mlConf.LogOutput = io.Discard
	}
	mlConf.PushPullInterval = conf.SyncInterval
	mlConf.BindPort = conf.BindPort
	mlConf.AdvertisePort = conf.BindPort
	mlConf.Name = string(s.node)
	mlConf.Delegate = &delegate{store: s}

	list, err := memberlist.Create(mlConf)
	if err != nil {
		return nil, err
	}

	seeds, err := conf.Discoverer.Discover(ctx)
	if err != nil {
		return nil, err
	}

	if len(seeds) > 0 {
		// TODO: error?
		_, _ = list.Join(seeds)
	}

	return list, nil
}
