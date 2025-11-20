package wsucana

import (
	"fmt"
	"sync"
	"time"

	serial "github.com/albenik/go-serial"
)

type aserial struct {
	*UsbCanA
	serial.Port
	sync.WaitGroup
	// sync.Mutex
}

const (
	FRAME_FIX_LEN      = 20
	FRAME_HEAD    byte = 0xAA
	FRAME_TAIL    byte = 0x55
)

func (my *aserial) init(ucan *UsbCanA) *aserial {
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
	my.Port.SetReadTimeoutEx(20, 1)

	return nil
}

func (my *aserial) close(out chan []byte) {
	out <- []byte("exit")
	my.Port.Close()
}

func (my *aserial) WaitClose() {
	my.WaitGroup.Wait()
}

func (my *aserial) startTransmit() {
	my.WaitGroup.Add(1)
	go my.transmit()

	// my.WaitGroup.Add(1)
	// go my.writeFrame(out)
}

func (my *aserial) writeRaw(data []byte) (int, error) {
	// my.Mutex.Lock()
	// defer my.Mutex.Unlock()
	return my.Port.Write(data)
}

// func (my *aserial) writeFrame(out <-chan []byte) {
// 	noexit := true
// 	for noexit {
// 		event := <-out
// 		switch event[0] {
// 		case FRAME_HEAD:
// 			for i := 10; i > 0; i-- {
// 				n, _ := my.writeRaw(event)
// 				// 如果写出不完整则续写，最多重试 10 次。
// 				if n < len(event) {
// 					event = event[n:]
// 				} else {
// 					break
// 				}
// 				time.Sleep(1 * time.Millisecond)
// 			}
// 			// 分组写入间隔 2ms。
// 			time.Sleep(2 * time.Millisecond)
// 		default:
// 			noexit = false
// 		}
// 	}
// 	my.WaitGroup.Done()
// }

func (my *aserial) writeFrame() {
	select {
	case event := <-my.out:
		switch event[0] {
		case FRAME_HEAD:
			for i := 10; i > 0; i-- {
				n, _ := my.writeRaw(event)
				// 如果写出不完整则续写，最多重试 10 次。
				if n < len(event) {
					event = event[n:]
				} else {
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
			// 分组写入间隔 2ms。
			time.Sleep(2 * time.Millisecond)
		default:
			// 其他事件直接忽略。
		}
	default:
		// 实现非阻塞式查询读取通道
	}
}

func (my *aserial) transmit() {
	var ob []byte
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
			if s <= 0 {
				my.writeFrame()
				continue ASerial_ReadAll_Main_Loop
			}

			ob = append(ob, rb[:s]...)
			for i := 0; i < len(ob); i++ {
				if ob[i] == FRAME_HEAD {
					if len(ob) < (i + 2) {
						// 长度不够则续读。
						ob = ob[i:]
						continue ASerial_ReadAll_Main_Loop
					}
					fl := my.UsbCanA.frameLen(ob[i:])
					if len(ob) < (i + fl) {
						// 长度不够则续读。
						ob = ob[i:]
						continue ASerial_ReadAll_Main_Loop
					}

					if ob[i+fl-1] == FRAME_TAIL {
						frame := my.UsbCanA.Unmarshal(ob[i : i+fl])
						select {
						case my.in <- *frame:
						default:
							println("in queue is full.")
						}
						i += fl - 1 // for 循环本身会增加 i 的值，所以这里需要减去 1，虽然会浪费 CPU 计算时间，但程序结构更好，逻辑更情绪，不容易漏加导致死循环。
					}
				}
			}
			// 能执行到这里，说明 ob 中的有效报文已经处理完，剩余的每个字节都匹配了，但是没有找到有效的报文/报文起始部分，可以清除 ob 中剩余的全部内容。
			ob = nil
		}
	}

	rb = nil
	// sysInfo = "端口失效，请选择其他端口"
	my.WaitGroup.Done()
}
