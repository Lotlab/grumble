package mumbleproto

import (
	"encoding/binary"

	"google.golang.org/protobuf/proto"
	"mumble.info/grumble/pkg/packetdata"
)

type UDPPacketType int

const (
	UDPPacketAudio UDPPacketType = iota
	UDPPacketPing
)

type AudioCodec int

const (
	CodecOpus AudioCodec = iota
	CodecCELTAlpha
	CodecCELTBeta
	CodecSpeex
)

type AudioTarget int

const (
	TargetRegularSpeech  AudioTarget = 0
	TargetServerLoopback AudioTarget = 31
)

type AudioContext int

const (
	ContextNormal = iota
	ContextShout
	ContextWhisper
	ContextListen
)

// UDPPacket is a generic form of parsed UDP packet
type UDPPacket interface {
	// LegacyData will encode the packet into the legacy format data
	LegacyData() []byte
	// ProtobufData will encode the packet into the protobuf format data
	ProtobufData() ([]byte, error)
	// Data will encode the packet into legacy or protobuf format data
	Data(legacy bool) ([]byte, error)
	// SetSenderSession sets the sender session, if capable
	SetSenderSession(session uint32)
}

type PingPacket struct {
	PingUDP
}

func (p *PingPacket) SetSenderSession(session uint32) {
}
func (p *PingPacket) LegacyData() []byte {
	buffer := make([]byte, 1)
	buffer[0] = (UDPMessagePing << 5)
	buffer = binary.LittleEndian.AppendUint32(buffer, 0)
	buffer = binary.LittleEndian.AppendUint64(buffer, p.GetTimestamp())
	return buffer
}

func (p *PingPacket) ProtobufData() ([]byte, error) {
	buffer := make([]byte, 1)
	buffer[0] = byte(UDPPacketPing)
	data, err := proto.Marshal(&p.PingUDP)
	if err != nil {
		return nil, err
	}
	buffer = append(buffer, data...)
	return buffer, nil
}
func (p *PingPacket) Data(legacy bool) ([]byte, error) {
	if legacy {
		return p.LegacyData(), nil
	}
	return p.ProtobufData()
}

type AudioPacket struct {
	AudioUDP

	UsedCodec       AudioCodec
	TargetOrContext uint8
	Payload         []byte
}

func (p *AudioPacket) SetSenderSession(session uint32) {
	p.SenderSession = session
}
func (p *AudioPacket) LegacyData() []byte {
	buffer := make([]byte, 32+len(p.Payload))
	switch p.UsedCodec {
	case CodecCELTAlpha:
		buffer[0] = (UDPMessageVoiceCELTAlpha << 5) | p.TargetOrContext
	case CodecCELTBeta:
		buffer[0] = (UDPMessageVoiceCELTBeta << 5) | p.TargetOrContext
	case CodecSpeex:
		buffer[0] = (UDPMessageVoiceSpeex << 5) | p.TargetOrContext
	case CodecOpus:
		buffer[0] = (UDPMessageVoiceOpus << 5) | p.TargetOrContext
	default:
		panic("unreachable")
	}
	outgoing := packetdata.New(buffer[1:])
	outgoing.PutUint32(p.SenderSession)
	outgoing.PutUint32(uint32(p.FrameNumber))

	switch p.UsedCodec {
	case CodecCELTAlpha:
		fallthrough
	case CodecCELTBeta:
		fallthrough
	case CodecSpeex:
		outgoing.CopyBytes(p.Payload)
	case CodecOpus:
		flag := len(p.Payload)
		if p.IsTerminator {
			flag |= 0x2000
		}
		outgoing.PutUint32(uint32(flag))
		outgoing.CopyBytes(p.Payload)
	}

	// Positional data
	if len(p.PositionalData) == 3 {
		for _, v := range p.PositionalData {
			outgoing.PutFloat32(v)
		}
	}

	return buffer[:1+outgoing.Size()]
}

func (p *AudioPacket) ProtobufData() ([]byte, error) {
	// At the moment only Opus is supported in the newer Protobuf UDP protocol
	// if the encoding is different, we automatically fall back to the legacy package format.
	if p.UsedCodec != CodecOpus {
		return p.LegacyData(), nil
	}

	buffer := make([]byte, 1)
	buffer[0] = byte(UDPPacketPing)
	data, err := proto.Marshal(&p.AudioUDP)
	if err != nil {
		return nil, err
	}
	buffer = append(buffer, data...)
	return buffer, nil
}

func (p *AudioPacket) Data(legacy bool) ([]byte, error) {
	if legacy {
		return p.LegacyData(), nil
	}
	return p.ProtobufData()
}

// PacketType returns the type of udp packet
func PacketType(pkt UDPPacket) UDPPacketType {
	switch pkt.(type) {
	case *PingPacket:
		return UDPPacketPing
	case *AudioPacket:
		return UDPPacketAudio
	default:
		panic("unreachable")
	}
}

// ParseUDPPacket parse the input UDP packet data into a UDPPacket, return if it's a legacy packet format
func ParseUDPPacket(data []byte, isLegacy bool) (pkt UDPPacket, legacy bool) {
	if len(data) < 1 {
		return nil, isLegacy
	}

	header := data[0]
	if isLegacy {
		if header == byte(UDPPacketPing) {
			pkt = parsePingPacketProtobuf(data[1:])
			legacy = false
			return
		}

		// This might be a legacy ping that requests additional information (they don't come with a header)
		if len(data) == 12 || len(data) == 24 {
			packet := parsePingPacketLegacy(data)
			if packet != nil {
				return packet, true
			}
		}

		kind := (header >> 5) & 0x07
		switch kind {
		case UDPMessagePing:
			return parsePingPacketLegacy(data[1:]), true
		case UDPMessageVoiceSpeex:
			return parseAudioPacketLegacy(data[1:], CodecSpeex), true
		case UDPMessageVoiceCELTAlpha:
			return parseAudioPacketLegacy(data[1:], CodecCELTAlpha), true
		case UDPMessageVoiceCELTBeta:
			return parseAudioPacketLegacy(data[1:], CodecCELTBeta), true
		case UDPMessageVoiceOpus:
			return parseAudioPacketLegacy(data[1:], CodecOpus), true
		}
	} else {
		switch header {
		case byte(UDPPacketPing):
			return parsePingPacketProtobuf(data[1:]), false
		case byte(UDPPacketAudio):
			return parseAudioPacketProtobuf(data[1:]), false
		}
	}

	return nil, isLegacy
}

func parsePingPacketLegacy(data []byte) *PingPacket {
	if len(data) != 12 || binary.LittleEndian.Uint32(data) != 0 {
		return nil
	}

	// Extended information ping request message. When received by the server, the message contains 4
	// leading, blank bytes followed by a 64bit client-specific timestamp. Thus, the only meaningful
	// decoding to do right now, is reading out the timestamp. Note that the byte-order of this field
	// (and its contents in general) is unspecified and thus the server code should never try to make
	// sense of it.
	ping := PingPacket{}
	ping.Timestamp = binary.LittleEndian.Uint64(data[4:])
	ping.RequestExtendedInformation = true
	return &ping
}

func parsePingPacketProtobuf(data []byte) *PingPacket {
	var ping PingPacket
	err := proto.Unmarshal(data, &ping.PingUDP)
	if err != nil {
		return nil
	}
	return &ping
}

func parseAudioPacketLegacy(data []byte, codec AudioCodec) *AudioPacket {
	if len(data) < 3 {
		return nil
	}

	var audio AudioPacket
	audio.UsedCodec = codec
	audio.TargetOrContext = data[0] & 0x1f

	incoming := packetdata.New(data[1:])
	audio.FrameNumber = incoming.GetUint64()

	offset := incoming.Size()
	payloadSize := 0

	switch codec {
	case CodecSpeex:
		fallthrough
	case CodecCELTAlpha:
		fallthrough
	case CodecCELTBeta:
		// For these old codecs, multiple frames may be sent as one payload. Each frame is started by a TOC byte
		// which encodes the length of the following frame and whether there will be a frame after it. The
		// length is encoded as the 7 least significant bits (0x7f) whereas the continuation flag is encoded in
		// the most significant bit (0x80).
		offset = incoming.Size()
		for {
			flag := incoming.Next8()
			frameSize := int(flag & 0x7f)

			if frameSize == 0 {
				audio.IsTerminator = true
			}

			payloadSize += frameSize
			incoming.Skip(frameSize)

			if flag&0x80 == 0 || !incoming.IsValid() {
				break
			}
		}
	case CodecOpus:
		size := int(incoming.GetUint16())
		payloadSize = size & 0x1fff
		audio.IsTerminator = size&0x2000 > 0
		incoming.Skip(payloadSize)
		offset = incoming.Size()
	}

	if !incoming.IsValid() {
		return nil
	}

	audio.Payload = data[1+offset : 1+offset+payloadSize]

	if incoming.Left() == 3*4 {
		// If there are further bytes after the audio payload, this means that there is positional data attached to
		// the packet.
		audio.PositionalData = make([]float32, 3)
		for i := 0; i < len(audio.PositionalData); i++ {
			audio.PositionalData[i] = incoming.GetFloat32()
		}
	} else if incoming.Left() > 0 {
		// The remaining data does not fit the size of positional data -> seems like a invalid package format
		return nil
	}
	// Legacy audio packets don't contain volume adjustments

	return &audio
}

func parseAudioPacketProtobuf(data []byte) *AudioPacket {
	var audio AudioPacket
	err := proto.Unmarshal(data, &audio.AudioUDP)
	if err != nil {
		return nil
	}
	audio.TargetOrContext = uint8(audio.GetTarget())
	// Atm the only codec supported by the new package format is Opus
	audio.UsedCodec = CodecOpus
	if len(audio.OpusData) == 0 {
		// Audio packets without audio data are invalid
		return nil
	}
	audio.Payload = audio.OpusData
	return &audio
}
