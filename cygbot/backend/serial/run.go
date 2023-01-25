package serial

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.bug.st/serial"
	"strconv"
	"strings"
	"unsafe"
)

type SerialType int

const (
	TYPE_SENSOR = iota
	TYPE_SERVO
)

type SerialConnection struct {
	ctx        context.Context
	port       string
	serialType SerialType
	callback   func(*SerialConnection, []byte)
	isRunning  bool
	isCancel   bool
	serialPort *serial.Port
}

func NewSerialConnection(ctx context.Context, port string, serialType SerialType, callback func(*SerialConnection, []byte)) *SerialConnection {
	return &SerialConnection{ctx: ctx, port: port, serialType: serialType, callback: callback}
}

func (sc *SerialConnection) Start(data0 int16) error {
	if sc.isRunning {
		return nil
	}

	sc.isRunning = true
	sc.isCancel = false

	var mode *serial.Mode
	if sc.serialType == TYPE_SENSOR {
		mode = &serial.Mode{
			BaudRate: 3000000,
			Parity:   serial.NoParity,
			DataBits: 8,
			StopBits: serial.OneStopBit,
		}
	} else if sc.serialType == TYPE_SERVO {
		mode = &serial.Mode{
			BaudRate: 115200,
			Parity:   serial.NoParity,
			DataBits: 8,
			StopBits: serial.OneStopBit,
		}
	}

	port, err := serial.Open(sc.port, mode)
	if err != nil {
		return err
	}
	sc.serialPort = &port

	if sc.serialType == TYPE_SENSOR {
		// Start 3D Mode
		_, err := port.Write([]byte{0x5A, 0x77, 0xFF, 0x02, 0x00, 0x08, 0x00, 0x0A})
		if err != nil {
			return err
		}
	} else if sc.serialType == TYPE_SERVO {
		_, err := port.Write([]byte(fmt.Sprintf("! %d\n", data0)))
		if err != nil {
			return err
		}
	}

	go sc.run()
	return nil
}

func (sc *SerialConnection) Stop() error {
	runtime.LogInfo(sc.ctx, "Stopping Serial Connection")

	sc.isRunning = false
	sc.isCancel = true

	if sc.serialPort != nil {
		if sc.serialType == TYPE_SENSOR {
			// Stop
			_, _ = (*sc.serialPort).Write([]byte{0x5A, 0x77, 0xFF, 0x02, 0x00, 0x02, 0x00, 0x00})
		} else if sc.serialType == TYPE_SERVO {
			_, _ = (*sc.serialPort).Write([]byte(fmt.Sprintf("! %d\n", 0)))
		}

		err := (*sc.serialPort).Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (sc *SerialConnection) run() {
	var threeDBuff []byte
	buf := make([]byte, 512)
	stepCnt := uint16(0)
	packetLength := uint16(0)
	headerStep := 0
	packetStep := 0
	stringBuffer := bytes.NewBufferString("")
	stringBuffer.Grow(1024)
	for {
		if sc.isCancel {
			return
		}
		n, err := (*sc.serialPort).Read(buf)
		if n == 0 || err != nil {
			continue
		}

		if sc.serialType == TYPE_SENSOR {
			for i := 0; i < n; i++ {
				// packetStep contains the state machine for parsing the stream data
				// https://cdn.sparkfun.com/assets/3/d/5/4/c/f5911d_726a54fc4f6644bcbec0d9b00236ffda.pdf
				if packetStep == 0 {
					if headerStep == 0 && buf[i] == 0x5A {
						headerStep = 1
					} else if headerStep == 1 && buf[i] == 0x77 {
						headerStep = 2
					} else if headerStep == 2 && buf[i] == 0xFF {
						headerStep = 0
						packetStep = 1
					} else {
						headerStep = 0
					}
				} else if packetStep == 1 {
					// Payload Length LSB
					packetLength = uint16(buf[i])
					packetStep = 2
				} else if packetStep == 2 {
					// Payload Length MSB
					packetLength = ((uint16(buf[i]) << 8) & 0xff00) | (packetLength & 0x00ff)
					stepCnt = uint16(0)
					threeDBuff = make([]byte, packetLength)
					packetStep = 3
				} else if packetStep == 3 {
					// Payload Length MSB
					if buf[i] != 0x08 { // 0x08 is 3d mode payload, do not accept any other
						packetStep = 0
					}
					packetStep = 4
				} else if packetStep == 4 {
					threeDBuff[stepCnt] = buf[i]
					stepCnt++
					packetLength--
					if packetLength <= 0 {
						sc.callback(sc, threeDBuff)
						packetStep = 0
					}
				}
			}
		} else if sc.serialType == TYPE_SERVO {
			stringBuffer.WriteString(string(buf[:n]))

			if strings.Contains(stringBuffer.String(), "\n") {
				s, err := stringBuffer.ReadString('\n')
				if err != nil {
					continue
				}
				cleanText := strings.Replace(s, "\r", "", -1)
				cleanText = strings.Replace(cleanText, "\n", "", -1)
				cleanText = strings.Replace(cleanText, "#", "", -1)
				cleanText = strings.Replace(cleanText, "°", "", -1)
				cleanText = strings.Trim(cleanText, " ")

				parseInt, err := strconv.ParseInt(cleanText, 10, 16)
				if err != nil {
					continue
				}
				uparseInt := uint16(parseInt)
				threeDBuff = make([]byte, unsafe.Sizeof(uparseInt))
				binary.LittleEndian.PutUint16(threeDBuff, uparseInt)
				sc.callback(sc, threeDBuff)
			}
		}
	}
}
