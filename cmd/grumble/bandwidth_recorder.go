package main

import "time"

const bandwidthWindowSize = 360

type BandwidthRecorder struct {
	enterTime    time.Time
	lastIdleTime time.Time

	packetTime       []time.Time
	packetSize       []uint64
	packetIndex      int
	windowPacketSize uint64
}

// NewBandwidthRecorder create a empty bandwidth recorder
func NewBandwidthRecorder() *BandwidthRecorder {
	return &BandwidthRecorder{
		enterTime:        time.Now(),
		lastIdleTime:     time.Time{},
		packetTime:       make([]time.Time, bandwidthWindowSize),
		packetSize:       make([]uint64, bandwidthWindowSize),
		packetIndex:      0,
		windowPacketSize: 0,
	}
}

// Bandwidth is the user's bandwidth of last second
func (b *BandwidthRecorder) Bandwidth() uint64 {
	sum := uint64(0)
	elapsed := time.Duration(0)

	// Get total size in last seconds
	for i := 1; i < bandwidthWindowSize; i++ {
		index := b.prevIndex(i)
		t := b.packetTime[index]
		if t.IsZero() {
			break
		}
		dt := time.Since(t)
		if dt > time.Second {
			break
		}
		sum += b.packetSize[i]
		elapsed = dt
	}

	if elapsed < 250*time.Millisecond {
		return 0
	}

	return sum * 1000 / uint64(elapsed.Milliseconds())
}

func (b *BandwidthRecorder) prevIndex(i int) int {
	return (b.packetIndex + bandwidthWindowSize - i) % bandwidthWindowSize
}

// OnlineSeconds is the duration since user connect to the server
func (b *BandwidthRecorder) OnlineSeconds() uint {
	return uint(time.Since(b.enterTime).Seconds())
}

// IdleSeconds is the duration since last active
func (b *BandwidthRecorder) IdleSeconds() uint {
	lastIndex := b.prevIndex(1)

	lastTime := b.packetTime[lastIndex]
	if lastTime.IsZero() {
		return uint(time.Since(b.enterTime).Seconds())
	}

	elapsed := time.Since(b.enterTime)
	if !b.lastIdleTime.IsZero() {
		idleElapsed := time.Since(b.lastIdleTime)
		if idleElapsed < elapsed {
			elapsed = idleElapsed
		}
	}

	return uint(elapsed.Seconds())
}

// AddFrame will check if user reach bandwidth limit, returning true if pass, false if limited
func (b *BandwidthRecorder) AddFrame(size int, limit int) bool {
	lastTime := b.packetTime[b.packetIndex]
	if lastTime.IsZero() {
		// Window is not full yet, use enter time as last time
		lastTime = b.enterTime
	}
	elapsed := time.Since(lastTime)

	if elapsed == 0 {
		return false
	}

	sum := b.windowPacketSize - b.packetSize[b.packetIndex] + uint64(size)
	bandwidth := sum * 1000 / uint64(elapsed.Milliseconds())
	if bandwidth >= uint64(limit) {
		return false
	}

	b.packetSize[b.packetIndex] = uint64(size)
	b.packetTime[b.packetIndex] = time.Now()
	b.windowPacketSize = sum

	b.packetIndex++
	if b.packetIndex >= bandwidthWindowSize {
		b.packetIndex = 0
	}

	return true
}

// ResetIdleSeconds will manually reset user's idle time
func (b *BandwidthRecorder) ResetIdleSeconds() {
	b.lastIdleTime = time.Now()
}
