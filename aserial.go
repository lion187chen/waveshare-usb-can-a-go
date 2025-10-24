package wsucana

import (
	"fmt"
	"sync"
	"time"

	"github.com/lion187chen/socketcan-go/canframe"
	serial "go.bug.st/serial"
)

type aserial struct {
	*UsbCanA
	serial.Port
	sync.WaitGroup
	sync.Mutex
	exit  bool
	group chan []byte
}

const (
	FRAME_FIX_LEN      = 20
	FRAME_HEAD    byte = 0xAA
	FRAME_TAIL    byte = 0x55
)

func (my *aserial) init(ucan *UsbCanA) *aserial {
	my.exit = false
	my.group = make(chan []byte, 1024)
	my.UsbCanA = ucan
	return my
}

func (my *aserial) open(port string) error {
	mode := &serial.Mode{
		BaudRate: 2000000,
		// ReadTimeout: time.Millisecond * 100,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	var e error
	my.Port, e = serial.Open(port, mode)
	if e != nil {
		return e
	}
	// Blocking mode.
	my.Port.SetReadTimeout(100 * time.Millisecond)

	return nil
}

func (my *aserial) close(out chan []byte) {
	out <- []byte("exit")
	my.Port.Close()
	my.exit = true

}

func (my *aserial) WaitClose() {
	my.WaitGroup.Wait()
}

func (my *aserial) startRead(in chan canframe.Frame, out <-chan []byte) {
	my.WaitGroup.Add(1)
	go my.groupParse(in)

	my.WaitGroup.Add(1)
	go my.readAll()

	my.WaitGroup.Add(1)
	go my.writeFrame(out)
}

func (my *aserial) writeRaw(data []byte) {
	my.Mutex.Lock()
	defer my.Mutex.Unlock()
	my.Port.Write(data)
}

func (my *aserial) writeFrame(out <-chan []byte) {
	noexit := true
	for noexit {
		event := <-out
		switch event[0] {
		case FRAME_HEAD:
			my.writeRaw(event)
		default:
			noexit = false
		}
	}
	my.WaitGroup.Done()
}

func (my *aserial) groupParse(in chan canframe.Frame) {
	var ob []byte

ASerial_Group_Parse_Main_Loop:
	for !my.exit {
		rb := <-my.group
		ob = append(ob, rb...)
		for i := 0; i < len(ob); {

			if ob[i] == FRAME_HEAD {
				ob = ob[i:]
				if 2 < len(ob) {

					fl := my.UsbCanA.frameLen(ob)
					if len(ob) >= fl {
						if ob[fl-1] == FRAME_TAIL {
							frame := my.UsbCanA.Unmarshal(ob[:fl])
							select {
							case in <- *frame:
							default:
								println("in queue is full.")
							}
							ob = ob[fl:]
							i = 0
						} else {
							// 不是有效的帧头，抛弃。
							ob = ob[i+1:]
							i = 0
							continue
						}
					} else {
						continue ASerial_Group_Parse_Main_Loop
					}

				} else {
					continue ASerial_Group_Parse_Main_Loop
				}
			} else {
				i++
			}
		}
	}
	my.WaitGroup.Done()
}

func (my *aserial) readAll() {
	noexit := true

	rb := make([]byte, 4096)

ASerial_ReadAll_Main_Loop:
	for noexit {
		s, e := my.Read(rb)

		if e != nil {
			fmt.Println(e.Error())
			switch err := e.(type) {
			case *serial.PortError:
				switch err.Code() {
				case serial.PortClosed:
					noexit = false
				case serial.InvalidSerialPort:
					noexit = false
				}
			}
			continue ASerial_ReadAll_Main_Loop
		} else {
			if s == 0 {
				continue ASerial_ReadAll_Main_Loop
			}

			select {
			case my.group <- rb[:s]:
			default:
				println("group queue is full.")
			}
		}
	}

	rb = nil
	// sysInfo = "端口失效，请选择其他端口"
	my.WaitGroup.Done()
}
