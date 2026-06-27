package peerlimit

import (
	"github.com/hashicorp/memberlist"
)

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

func (d *delegate) LocalState(_ bool) []byte {
	crdt, err := d.store.snapshot()
	if err != nil {
		return nil
	}
	return crdt
}

func (d *delegate) MergeRemoteState(buf []byte, _ bool) {
	crdt, err := unmarshalCRDT(buf)
	if err != nil {
		return
	}
	d.store.merge(crdt)
}

func startGossip(s *store, conf Config) (*memberlist.Memberlist, error) {
	mlConf := memberlist.DefaultLANConfig()
	mlConf.PushPullInterval = conf.SyncInterval
	mlConf.BindPort = conf.BindPort
	mlConf.AdvertisePort = conf.BindPort
	mlConf.Name = string(s.node)
	mlConf.Delegate = &delegate{store: s}

	list, err := memberlist.Create(mlConf)
	if err != nil {
		return nil, err
	}

	if len(conf.Seeds) > 0 {
		_, err := list.Join(conf.Seeds)
		if err != nil {
			return nil, err
		}
	}

	return list, nil
}
