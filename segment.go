package atp

import (
	"encoding/binary"
	"time"
)

var segmentMtu = defaultMTU

const (
	ackDelimStart byte = 1
	ackDelimSeq   byte = 2
	ackDelimRange byte = 4
	ackDelimEnd   byte = 8
)

const (
	flagACK byte = 1
	flagSYN byte = 2

	// segments sent with this flag are subject to retransmission and may
	// not be used to measure RTT
	flagRTO byte = 8
)

const defaultRetransmitThresh = 3

var dataOffsetPosition = position{0, 1}
var flagPosition = position{1, 2}
var sequenceNumberPosition = position{2, 6}
var windowSizePosition = position{6, 10}

func getDataChunkSize() int {
	return segmentMtu - headerLength
}

func bytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}

func uint32ToBytes(data uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, data)
	return result
}

func isFlaggedAs(input byte, flag byte) bool {
	return input&flag == flag
}

type segment struct {
	buffer           []byte
	sequenceNumber   []byte
	windowSize       []byte
	data             []byte
	timestamp        time.Time
	retransmitThresh uint32
}

func (seg *segment) getDataOffset() byte {
	return seg.buffer[dataOffsetPosition.Start]
}

func (seg *segment) getHeaderSize() int {
	return int(seg.getDataOffset())
}

func (seg *segment) addFlag(flag byte) {
	seg.setFlags(seg.getFlags() | flag)
}

func (seg *segment) getFlags() byte {
	return seg.buffer[flagPosition.Start]
}

func (seg *segment) setFlags(flags byte) {
	seg.buffer[flagPosition.Start] = flags
}

func (seg *segment) isFlaggedAs(flag byte) bool {
	return isFlaggedAs(seg.getFlags(), flag)
}

func (seg *segment) getSequenceNumber() uint32 {
	return bytesToUint32(seg.sequenceNumber)
}

func (seg *segment) getWindowSize() uint32 {
	return bytesToUint32(seg.windowSize)
}

func (seg *segment) setWindowSize(windowSize uint32) {
	seg.windowSize = seg.buffer[windowSizePosition.Start:windowSizePosition.End]
	binary.BigEndian.PutUint32(seg.windowSize, windowSize)
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[dataOffsetPosition.Start] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[flagPosition.Start] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End], sequenceNumber)
}

func createSegment(buffer []byte) *segment {
	dataOffset := int(buffer[dataOffsetPosition.Start])
	flag := buffer[flagPosition.Start]
	seg := &segment{
		buffer:           buffer,
		sequenceNumber:   buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End],
		data:             buffer[dataOffset:],
		retransmitThresh: defaultRetransmitThresh,
	}
	if isFlaggedAs(flag, flagACK) {
		seg.windowSize = buffer[windowSizePosition.Start:windowSizePosition.End]
	}
	return seg
}

func getDataOffsetForFlag(flag byte) int {
	if isFlaggedAs(flag, flagACK) {
		return windowSizePosition.End
	}
	return sequenceNumberPosition.End
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) *segment {
	dataOffset := getDataOffsetForFlag(flags)
	buffer := make([]byte, dataOffset+len(data))
	setDataOffset(buffer, byte(dataOffset))
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(lastInOrder, sequenceNumber, windowSize uint32) *segment {
	seg := createFlaggedSegment(lastInOrder, flagACK, uint32ToBytes(sequenceNumber))
	seg.setWindowSize(windowSize)
	return seg
}

func insertSegmentInOrder(segments []*segment, insert *segment) []*segment {
	for i, seg := range segments {
		if insert.getSequenceNumber() < seg.getSequenceNumber() {
			segments = append(segments, nil)
			copy(segments[i+1:], segments[i:])
			segments[i] = insert
			return segments
		}
		if insert.getSequenceNumber() == seg.getSequenceNumber() {
			return segments
		}
	}
	return append(segments, insert)
}

func removeSegment(segments []*segment, sequenceNumber uint32) (*segment, []*segment) {
	for i, seg := range segments {
		if seg.getSequenceNumber() == sequenceNumber {
			return seg, append(segments[:i], segments[i+1:]...)
		}
	}
	return nil, segments
}

func removeAllSegmentsWhere(segments []*segment, condition func(*segment) bool) (removed []*segment, orig []*segment) {
	removed = make([]*segment, 0, len(segments))
	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		if condition(seg) {
			segments = append(segments[:i], segments[i+1:]...)
			removed = append(removed, seg)
			i--
		}
	}
	return removed, segments
}

func popSegment(segments []*segment) (*segment, []*segment) {
	return segments[0], segments[1:]
}
