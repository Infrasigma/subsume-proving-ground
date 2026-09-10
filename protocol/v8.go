package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	MagicV8   = "CWLTREC2"
	VersionV8 = uint32(2)

	HeaderSize = 68
	RecordSize = 24660
	FrameBytes = 64 * 64 * 3
	ActionSize = 64
	RecordMetadataSize = 84
)

type DatasetHeaderV8 struct {
	Magic       [8]byte
	Version     uint32
	RecordCount uint32
	RecordSize  uint32
	Seed        uint64
	ClassCounts [8]uint32
	Reserved    [8]byte
}

type ActionRecord struct {
	Type     uint32
	Params   [8]int32
	Reserved [28]byte
}

type TransitionRecordV8 struct {
	TickBefore      uint64
	TickAfter       uint64
	Action          ActionRecord
	ConsequenceMask uint8
	Padding         [3]byte
	FrameBefore     [FrameBytes]byte
	FrameAfter      [FrameBytes]byte
}

func init() {
	if uint32(binary.Size(DatasetHeaderV8{})) != HeaderSize {
		panic("protocol: DatasetHeaderV8 size mismatch")
	}
	if uint32(binary.Size(ActionRecord{})) != ActionSize {
		panic("protocol: ActionRecord size mismatch")
	}
	if uint32(binary.Size(TransitionRecordV8{})) != RecordSize {
		panic("protocol: TransitionRecordV8 size mismatch")
	}
}

func NewDatasetHeaderV8(seed uint64, recordCount uint32, classCounts [8]uint32) DatasetHeaderV8 {
	var h DatasetHeaderV8
	copy(h.Magic[:], MagicV8)
	h.Version = VersionV8
	h.RecordCount = recordCount
	h.RecordSize = RecordSize
	h.Seed = seed
	h.ClassCounts = classCounts
	return h
}

func (h DatasetHeaderV8) Validate() error {
	if string(h.Magic[:]) != MagicV8 {
		return fmt.Errorf("invalid magic %q", string(h.Magic[:]))
	}
	if h.Version != VersionV8 {
		return fmt.Errorf("unsupported version %d", h.Version)
	}
	if h.RecordSize != RecordSize {
		return fmt.Errorf("invalid record size %d, want %d", h.RecordSize, RecordSize)
	}
	return nil
}

func WriteHeaderV8(w io.Writer, h *DatasetHeaderV8) error {
	if err := h.Validate(); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, h)
}

func WriteRecordV8(w io.Writer, r *TransitionRecordV8) error {
	if r.Padding != [3]byte{} {
		return fmt.Errorf("record padding must be zero")
	}
	return binary.Write(w, binary.LittleEndian, r)
}

func RecordOffsetV8(index uint32) int64 {
	return int64(HeaderSize) + int64(index)*int64(RecordSize)
}

func ValidateMask(mask uint8) error {
	// All 8 bits are valid consequence classes; uint8 provides the complete domain.
	_ = mask
	return nil
}
