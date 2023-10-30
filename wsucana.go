package wsucana

import (
	"encoding/binary"
	"time"

	"github.com/lion187chen/socketcan-go/canframe"
)

type UsbCanA struct {
	*aserial
	in  chan canframe.Frame
	out chan []byte
}

const (
	FRAME_CFG_NO_VARIABLE byte = 0x02
	FRAME_CFG_VARIABLE    byte = 0x12
)

type BiterateType byte

const (
	FRAME_CFG_BIT_RATE_1M BiterateType = iota + 1
	FRAME_CFG_BIT_RATE_800K
	FRAME_CFG_BIT_RATE_500K
	FRAME_CFG_BIT_RATE_400K
	FRAME_CFG_BIT_RATE_250K
	FRAME_CFG_BIT_RATE_200K
	FRAME_CFG_BIT_RATE_125K
	FRAME_CFG_BIT_RATE_100K
	FRAME_CFG_BIT_RATE_50K
	FRAME_CFG_BIT_RATE_20K
	FRAME_CFG_BIT_RATE_10K
	FRAME_CFG_BIT_RATE_5K
)

type CanFrameType byte

const (
	FRAME_CFG_CAN_FRAME_STD CanFrameType = iota + 1
	FRAME_CFG_CAN_FRAME_EXT
)

type WorkModeType byte

const (
	FRAME_CFG_WRK_MOD_NORMAL WorkModeType = iota
	FRAME_CFG_WRK_MOD_SILENCE
	FRAME_CFG_WRK_MOD_LOOP
	FRAME_CFG_WRK_MOD_LOOP_SILENCE
)

type RepeatType byte

const (
	FRAME_CFG_REPEAT_AUTO RepeatType = iota
	FRAME_CFG_REPEAT_NO
)

const (
	FRAME_DATA_MAX_LEN      int  = canframe.FRAME_MAX_DATA_LEN
	FRAME_DATA_FLAG_EXT     byte = 0x20
	FRAME_DATA_FLAG_RTR     byte = 0x10
	FRAME_DATA_VARIABLE_LEN byte = 0xC0
	FRAME_DATA_DLC_MASK     byte = 0x0F
)

func (my *UsbCanA) Open(port string, channelSize int) error {
	my.aserial = new(aserial).init(my)
	e := my.aserial.open(port)
	if e != nil {
		my.aserial = nil
		return e
	}
	my.in = make(chan canframe.Frame, channelSize)
	my.out = make(chan []byte, channelSize)
	my.aserial.startRead(my.in, my.out)
	return nil
}

func (my *UsbCanA) Close() {
	if my.aserial != nil {
		my.aserial.close(my.out)
		my.aserial.WaitClose()
		my.aserial = nil
	}
}

func (my *UsbCanA) Config(biterate BiterateType, framet CanFrameType, mode WorkModeType, repeat RepeatType) {
	frame := make([]byte, 20)
	frame[0] = FRAME_HEAD
	frame[1] = FRAME_TAIL
	frame[2] = FRAME_CFG_VARIABLE
	frame[3] = byte(biterate)
	frame[4] = byte(framet)
	frame[13] = byte(mode)
	frame[14] = byte(repeat)

	frame[19] = 0
	for i := 2; i < 19; i++ {
		frame[19] += frame[i]
	}

	my.aserial.writeRaw(frame)
	// 间隔 80ms 以上才能稳定通信.
	time.Sleep(100 * time.Millisecond)
}

func (my *UsbCanA) WriteFrame(frame *canframe.Frame) {
	my.out <- my.Marshal(frame)
}

func (my *UsbCanA) frameLen(data []byte) int {
	fl := 3
	if data[1]&FRAME_DATA_FLAG_EXT == FRAME_DATA_FLAG_EXT {
		fl += 4
	} else {
		fl += 2
	}
	fl += int(data[1] & FRAME_DATA_DLC_MASK)
	return fl
}

func (my *UsbCanA) Marshal(frame *canframe.Frame) []byte {
	if len(frame.Data) > FRAME_DATA_MAX_LEN {
		frame.Data = frame.Data[:FRAME_DATA_MAX_LEN]
	}

	bs := make([]byte, 2)
	bs[0] = FRAME_HEAD
	bs[1] |= FRAME_DATA_VARIABLE_LEN
	if frame.IsExtended {
		bs[1] |= FRAME_DATA_FLAG_EXT
		bs = binary.LittleEndian.AppendUint32(bs, frame.ID)
	} else {
		bs = binary.LittleEndian.AppendUint16(bs, uint16(frame.ID))
	}
	if frame.IsRemote {
		bs[1] |= FRAME_DATA_FLAG_RTR
	}
	bs[1] |= byte(len(frame.Data))
	bs = append(bs, frame.Data...)
	bs = append(bs, FRAME_TAIL)
	return bs
}

func (my *UsbCanA) Unmarshal(bs []byte) *canframe.Frame {
	var frame canframe.Frame
	frame.IsError = false

	if len(bs) < my.frameLen(bs) {
		return nil
	}

	if bs[1]&FRAME_DATA_FLAG_EXT == FRAME_DATA_FLAG_EXT {
		frame.ID = (uint32(bs[5]) << 24) | (uint32(bs[4]) << 16) | (uint32(bs[3]) << 8) | uint32(bs[2])
		frame.IsExtended = true
	} else {
		frame.ID = (uint32(bs[3]) << 8) | uint32(bs[2])
	}

	if bs[1]&FRAME_DATA_FLAG_RTR == FRAME_DATA_FLAG_RTR {
		frame.IsRemote = true
	}
	dlc := int(bs[1] & FRAME_DATA_DLC_MASK)
	for i := 0; i < dlc; i++ {
		if frame.IsExtended {
			frame.Data = append(frame.Data, bs[i+6])
		} else {
			frame.Data = append(frame.Data, bs[i+4])
		}
	}

	return &frame
}

func (my *UsbCanA) GetReadChannel() <-chan canframe.Frame {
	return my.in
}
