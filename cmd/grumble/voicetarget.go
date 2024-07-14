// Copyright (c) 2011 The Grumble Authors
// The use of this source code is goverened by a BSD-style
// license that can be found in the LICENSE-file.

package main

import (
	"mumble.info/grumble/pkg/acl"
	"mumble.info/grumble/pkg/mumbleproto"
)

// A VoiceTarget holds information about a single
// VoiceTarget entry of a Client.
type VoiceTarget struct {
	sessions []uint32
	channels []voiceTargetChannel

	directCache           map[uint32]*Client
	fromChannelsCache     map[uint32]*Client
	listeningTargetsCache map[uint32]*VolumeAdjustment
}

type voiceTargetChannel struct {
	id          uint32
	subChannels bool
	links       bool
	onlyGroup   string
}

// Add's a client's session to the VoiceTarget
func (vt *VoiceTarget) AddSession(session uint32) {
	vt.sessions = append(vt.sessions, session)
}

// AddChannel adds a channel to the VoiceTarget.
// If subchannels is true, any sent voice packets will also be sent to all subchannels.
// If links is true, any sent voice packets will also be sent to all linked channels.
// If group is a non-empty string, any sent voice packets will only be broadcast to members
// of that group who reside in the channel (or its children or linked channels).
func (vt *VoiceTarget) AddChannel(id uint32, subchannels bool, links bool, group string) {
	vt.channels = append(vt.channels, voiceTargetChannel{
		id:          id,
		subChannels: subchannels,
		links:       links,
		onlyGroup:   group,
	})
}

// IsEmpty checks whether the VoiceTarget is empty (has no targets)
func (vt *VoiceTarget) IsEmpty() bool {
	return len(vt.sessions) == 0 && len(vt.channels) == 0
}

// ClearCache clears the VoiceTarget's cache.
func (vt *VoiceTarget) ClearCache() {
	vt.directCache = nil
	vt.fromChannelsCache = nil
	vt.listeningTargetsCache = nil
}

// Send the contents of the VoiceBroadcast to all targets specified in the
// VoiceTarget.
func (vt *VoiceTarget) SendVoiceBroadcast(vb *VoiceBroadcast) {
	vt.tryRebuildCache(vb)

	for _, target := range vt.fromChannelsCache {
		vb.AddReceiver(target, mumbleproto.ContextShout, nil)
	}

	for _, target := range vt.directCache {
		vb.AddReceiver(target, mumbleproto.ContextWhisper, nil)
	}

	for session, adjustment := range vt.listeningTargetsCache {
		target, ok := vb.sender.server.clients[session]
		if !ok {
			continue
		}
		vb.AddReceiver(target, mumbleproto.ContextListen, adjustment)
	}
}

func (vt *VoiceTarget) tryRebuildCache(vb *VoiceBroadcast) {
	if vt.directCache != nil && vt.fromChannelsCache != nil && vt.listeningTargetsCache != nil {
		return
	}

	client := vb.sender
	server := client.server

	direct := make(map[uint32]*Client)
	fromChannels := make(map[uint32]*Client)
	listeningTargets := make(map[uint32]*VolumeAdjustment)

	for _, vtc := range vt.channels {
		channel := server.Channels[int(vtc.id)]
		if channel == nil {
			continue
		}

		if !vtc.subChannels && !vtc.links && vtc.onlyGroup == "" {
			if acl.HasPermission(&channel.ACL, client, acl.WhisperPermission) {
				for _, target := range channel.clients {
					fromChannels[target.Session()] = target
				}
				for _, target := range server.listenerManager.GetListenersForChannel(vtc.id) {
					volAdj := server.listenerManager.GetVolumeAdjustmentDefault(target, vtc.id)
					old, ok := listeningTargets[target]
					if ok && old.Factor > volAdj.Factor {
						volAdj = *old
					}
					listeningTargets[target] = &volAdj
				}
			}
		} else {
			server.Printf("%v", vtc)
			newchans := make(map[int]*Channel)
			if vtc.links {
				newchans = channel.AllLinks()
			} else {
				newchans[channel.Id] = channel
			}
			if vtc.subChannels {
				subchans := channel.AllSubChannels()
				for k, v := range subchans {
					newchans[k] = v
				}
			}

			// todo(jim-k): handle whisper redirect
			for _, newchan := range newchans {
				if acl.HasPermission(&newchan.ACL, client, acl.WhisperPermission) {
					for _, target := range newchan.clients {
						if vtc.onlyGroup == "" || acl.GroupMemberCheck(&newchan.ACL, &newchan.ACL, vtc.onlyGroup, target) {
							fromChannels[target.Session()] = target
						}
					}

					for _, id := range server.listenerManager.GetListenersForChannel(uint32(newchan.Id)) {
						target, ok := server.clients[id]
						if !ok {
							continue
						}
						if vtc.onlyGroup == "" || acl.GroupMemberCheck(&newchan.ACL, &newchan.ACL, vtc.onlyGroup, target) {
							volAdj := server.listenerManager.GetVolumeAdjustmentDefault(id, uint32(newchan.Id))
							old, ok := listeningTargets[id]
							if ok && old.Factor > volAdj.Factor {
								volAdj = *old
							}
							listeningTargets[id] = &volAdj
						}
					}
				}
			}
		}
	}

	for _, session := range vt.sessions {
		target := server.clients[session]
		if target != nil && acl.HasPermission(&target.Channel.ACL, target, acl.WhisperPermission) {
			if _, alreadyInFromChannels := fromChannels[target.Session()]; !alreadyInFromChannels {
				direct[target.Session()] = target
			}
		}
	}

	// Make sure the speaker themselves is not contained in these lists
	delete(direct, client.session)
	delete(fromChannels, client.session)
	delete(listeningTargets, client.session)

	vt.directCache = direct
	vt.fromChannelsCache = fromChannels
	vt.listeningTargetsCache = listeningTargets
}
