package main

type ClientVersion uint64

func VersionFromComponent(major, minor, patch uint16) ClientVersion {
	return ClientVersion((uint64(major) << 48) | (uint64(minor) << 32) | (uint64(patch) << 16))
}

func VersionFromV1(ver uint32) ClientVersion {
	return VersionFromComponent(uint16((ver>>16)&0xFFFF), uint16((ver>>8)&0xFF), uint16((ver>>0)&0xFF))
}

func (v ClientVersion) Major() uint16 {
	return uint16((uint64(v) >> 48) & 0xFFFF)
}

func (v ClientVersion) Minor() uint16 {
	return uint16((uint64(v) >> 32) & 0xFFFF)
}

func (v ClientVersion) Patch() uint16 {
	return uint16((uint64(v) >> 16) & 0xFFFF)
}

func (v ClientVersion) VersionV2() uint64 {
	return uint64(v)
}

func (v ClientVersion) VersionV1() uint32 {
	return uint32(v.Major())<<16 | uint32(v.Minor()&0xFF)<<8 | uint32(v.Patch()&0xFF)
}

func (v ClientVersion) SupportDescBlobHash() bool {
	return v >= VersionFromComponent(1, 2, 2)
}

func (v ClientVersion) SupportRecording() bool {
	return v >= VersionFromComponent(1, 2, 3)
}

func (v ClientVersion) SupportCommentTextureHash() bool {
	return v >= VersionFromComponent(1, 2, 3)
}

func (v ClientVersion) SendTextureDataInMessage() bool {
	return v < VersionFromComponent(1, 2, 2)
}

func (v ClientVersion) SupportProtobuf() bool {
	return v >= VersionFromComponent(1, 5, 0)
}
