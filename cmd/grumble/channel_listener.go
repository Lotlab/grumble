package main

type ChannelListener struct {
	session   uint32
	channelID uint32
}

type ChannelListenerManager struct {
	listeningUsers   map[uint32]map[uint32]bool
	listenedChannels map[uint32]map[uint32]bool
	listenerVolAdjs  map[ChannelListener]VolumeAdjustment
}

func NewChannelListenerManager() ChannelListenerManager {
	return ChannelListenerManager{
		listeningUsers:   make(map[uint32]map[uint32]bool),
		listenedChannels: make(map[uint32]map[uint32]bool),
		listenerVolAdjs:  make(map[ChannelListener]VolumeAdjustment),
	}
}

func (m *ChannelListenerManager) Add(session, channel uint32) {
	m.listeningUsers[session][channel] = true
	m.listenedChannels[channel][session] = true
}

func (m *ChannelListenerManager) Remove(session, channel uint32) {
	delete(m.listeningUsers[session], channel)
	delete(m.listenedChannels[channel], session)
}

func (m *ChannelListenerManager) IsListening(session, channel uint32) bool {
	arr, ok := m.listenedChannels[channel]
	if !ok {
		return false
	}
	_, ok = arr[channel]
	return ok
}

func (m *ChannelListenerManager) IsListeningToAny(session uint32) bool {
	return len(m.listeningUsers[session]) > 0
}

func (m *ChannelListenerManager) IsListenedByAny(channel uint32) bool {
	return len(m.listenedChannels[channel]) > 0
}

func (m *ChannelListenerManager) GetListenersForChannel(channel uint32) []uint32 {
	data, ok := m.listenedChannels[channel]
	if !ok {
		return nil
	}
	arr := make([]uint32, 0, len(data))
	for k := range data {
		arr = append(arr, k)
	}
	return arr
}

func (m *ChannelListenerManager) GetListenerCountForChannel(channel uint32) int {
	data := m.listenedChannels[channel]
	return len(data)
}

func (m *ChannelListenerManager) GetListenedChannelCountForUser(session uint32) int {
	data := m.listeningUsers[session]
	return len(data)
}

func (m *ChannelListenerManager) SetVolumeAdjustment(session, channel uint32, volAdj VolumeAdjustment) {
	m.listenerVolAdjs[ChannelListener{session, channel}] = volAdj
}

func (m *ChannelListenerManager) GetVolumeAdjustment(session, channel uint32) *VolumeAdjustment {
	val, ok := m.listenerVolAdjs[ChannelListener{session, channel}]
	if !ok {
		return nil
	}
	return &val
}

func (m *ChannelListenerManager) GetVolumeAdjustmentDefault(session, channel uint32) VolumeAdjustment {
	val, ok := m.listenerVolAdjs[ChannelListener{session, channel}]
	if !ok {
		return DefaultVolumeAdjustment()
	}
	return val
}
func (m *ChannelListenerManager) Clear() {
	for k := range m.listenedChannels {
		delete(m.listenedChannels, k)
	}
	for k := range m.listenedChannels {
		delete(m.listenedChannels, k)
	}
	for cl := range m.listenerVolAdjs {
		delete(m.listenerVolAdjs, cl)
	}
}
