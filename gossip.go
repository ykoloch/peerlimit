package peerlimit

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
