package main

import (
	"flag"
	"fmt"

	"github.com/lion187chen/socketcan-go/canframe"
	wsucana "github.com/lion187chen/waveshare-usb-can-a-go"
)

var ucan *wsucana.UsbCanA

func main() {
	var port string
	flag.StringVar(&port, "port", "COM7", "WaveShare USB-CAN-A's virtual serial port name.")

	ucan = new(wsucana.UsbCanA)
	// Open() will create two goroutines, one for serial tx, another for serial rx.
	// bufSize is the rx and tx channelSize size.
	ucan.Open(port, 16)
	// Config the Waveshare USB-CAN-A device first.
	ucan.Config(wsucana.FRAME_CFG_BIT_RATE_100K, wsucana.FRAME_CFG_CAN_FRAME_EXT, wsucana.FRAME_CFG_WRK_MOD_NORMAL, wsucana.FRAME_CFG_REPEAT_NO)

	go Read(ucan)

	var aFrame canframe.Frame = canframe.Frame{
		ID:         0x20,
		Data:       []byte{0x01, 0x02, 0x03, 0x4, 0x05, 0x06, 0x07, 0x08},
		IsExtended: true,
		IsRemote:   false,
		IsError:    false,
	}
	// WriteFrame() will packa CAN frame to serial packate and send it to serial tx goroutine.
	ucan.WriteFrame(&aFrame)

	// Wait the serial tx and rx goroutines exited.
	ucan.WaitClose()
}

func Read(ucan *wsucana.UsbCanA) {
	for {
		rframe := <-ucan.GetReadChannel()
		fmt.Println(rframe)
		// Close() will stop serial tx and rx goroutines.
		ucan.Close()
	}
}
