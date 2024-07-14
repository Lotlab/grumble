package main

import (
	"bytes"
	"math"
	"sort"

	"mumble.info/grumble/pkg/mumbleproto"
)

type VoiceBroadcast struct {
	// The sender who is performing the broadcast
	sender *Client
	// Audio packet which should be broadcast
	packet *mumbleproto.AudioPacket

	// Receivers
	receiver    []voiceReceiver
	receiverMap map[uint32]int
}

type voiceReceiver struct {
	Client           *Client
	HasPositional    bool
	Ctx              mumbleproto.AudioContext
	VolumeAdjustment *VolumeAdjustment
}

func (v *voiceReceiver) VolumeFactor() float32 {
	if v.VolumeAdjustment == nil {
		return 1
	}
	return v.VolumeAdjustment.Factor
}

func (v *voiceReceiver) VolumeDB() int {
	if v.VolumeAdjustment == nil {
		return 0
	}
	return v.VolumeAdjustment.DBAdjustment
}

func (v *voiceReceiver) SupportProtobuf() bool {
	return v.Client.Version.SupportProtobuf()
}

func (v *voiceReceiver) Equal(other *voiceReceiver) bool {
	return v.HasPositional == other.HasPositional &&
		v.SupportProtobuf() == other.SupportProtobuf() &&
		v.Ctx == other.Ctx &&
		math.Abs(float64(v.VolumeFactor()-other.VolumeFactor())) < 0.05 &&
		math.Abs(float64(v.VolumeDB()-other.VolumeDB())) < 5
}

func NewVoiceBroadcast(client *Client, packet *mumbleproto.AudioPacket) *VoiceBroadcast {
	return &VoiceBroadcast{
		sender:      client,
		packet:      packet,
		receiver:    make([]voiceReceiver, 0, 16),
		receiverMap: make(map[uint32]int),
	}
}

func (v *VoiceBroadcast) Target() byte {
	return v.packet.TargetOrContext
}

func (v *VoiceBroadcast) AddReceiver(client *Client, ctx mumbleproto.AudioContext, volAdj *VolumeAdjustment) {
	if client == nil {
		return
	}

	hasPositionalData := len(v.packet.PositionalData) == 3 && bytes.Equal(client.PluginContext, v.sender.PluginContext)
	receiver := voiceReceiver{
		Client:           client,
		HasPositional:    hasPositionalData,
		Ctx:              ctx,
		VolumeAdjustment: volAdj,
	}

	id, ok := v.receiverMap[client.session]
	if ok {
		// Update info.
		if receiver.Ctx < v.receiver[id].Ctx {
			v.receiver[id].Ctx = receiver.Ctx
			v.receiver[id].VolumeAdjustment = receiver.VolumeAdjustment
		}
		oldAdj := float32(1.0)
		newAdj := float32(1.0)
		if v.receiver[id].VolumeAdjustment != nil {
			oldAdj = v.receiver[id].VolumeAdjustment.Factor
		}
		if receiver.VolumeAdjustment != nil {
			newAdj = receiver.VolumeAdjustment.Factor
		}
		if newAdj < oldAdj {
			v.receiver[id].VolumeAdjustment = receiver.VolumeAdjustment
		}
	} else {
		id = len(v.receiver)
		v.receiver = append(v.receiver, receiver)
		v.receiverMap[client.session] = id
	}
}

func (v *VoiceBroadcast) Broadcast() error {
	// Sort by parameter
	sort.Slice(v.receiver, func(i, j int) bool {
		a := v.receiver[i]
		b := v.receiver[j]

		if a.HasPositional != b.HasPositional {
			return a.HasPositional
		}

		if a.SupportProtobuf() != b.SupportProtobuf() {
			return a.SupportProtobuf()
		}

		if a.Ctx != b.Ctx {
			return a.Ctx < b.Ctx
		}

		return a.VolumeFactor() > b.VolumeFactor()
	})

	var lastReceiver voiceReceiver
	var lastBytes []byte = nil

	for _, recv := range v.receiver {
		if lastBytes == nil || !lastReceiver.Equal(&recv) {
			if !recv.HasPositional {
				v.packet.PositionalData = nil
			}
			v.packet.TargetOrContext = uint8(recv.Ctx)
			volumeAdjustment := recv.VolumeFactor()
			if volumeAdjustment != 1 {
				v.packet.VolumeAdjustment = volumeAdjustment
			} else {
				v.packet.VolumeAdjustment = 0
			}

			bytes, err := v.packet.Data(!recv.SupportProtobuf())
			if err != nil {
				v.sender.server.Panic(err)
			} else {
				lastBytes = bytes
			}

			lastReceiver = recv
		}

		if err := recv.Client.SendUDP(lastBytes); err != nil {
			recv.Client.Panicf("Unable to send UDP message: %v", err.Error())
		}
	}

	return nil
}
