package backend

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	sc "github.com/Gurkengewuerz/ba-cygbot/backend/serial"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.bug.st/serial"
	"golang.org/x/image/math/f32"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

type Field3D f32.Vec4
type Result3D [][]Field3D
type Data3D []Result3D

const MM2M = float32(0.001)
const SENSOR_PIXEL_REAL_SIZE = float32(0.02) // in mm
const SENSOR_MAX_DEPTH_3D = 3000             // in mm
const SENSOR_COL = 160
const SENSOR_ROW = 60

// App struct
type App struct {
	ctx               context.Context
	lensTransformData []Field3D
	scanResult        Data3D
	currentDegrees    float64
	degreeChange      int64
	needRecord        bool
	servo             *sc.SerialConnection
	sensor            *sc.SerialConnection
	lastWrite         int64
	lastWriteRoom     int64
	isFixRoation      bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved,
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.initLensTransform()
	a.resetMatrix()
}

func (a *App) resetMatrix() {
	a.scanResult = make(Data3D, 180+1)
	for i := range a.scanResult {
		a.scanResult[i] = make(Result3D, SENSOR_ROW)
		for y := range a.scanResult[i] {
			a.scanResult[i][y] = make([]Field3D, SENSOR_COL)
		}
	}
}

func (a *App) initLensTransform() {
	a.lensTransformData = make([]Field3D, SENSOR_ROW*SENSOR_COL)

	r0 := 1 - (SENSOR_ROW / 2)
	c0 := 1 - (SENSOR_COL / 2)

	for y, row := 0, r0; y < SENSOR_ROW; row, y = row+1, y+1 {
		for x, col := 0, c0; x < SENSOR_COL; col, x = col+1, x+1 {
			c := float32(col) - 0.5
			r := float32(row) - 0.5
			angleGrad := a.getAngle(c, r)
			angleRad := float64(angleGrad * math.Pi / 180.0)

			rp := float32(math.Sqrt(float64((c * c) + (r * r))))
			rUA := float32(math.Sin(angleRad))

			idx := x + (y * SENSOR_COL)
			a.lensTransformData[idx][0] = c * rUA / rp
			a.lensTransformData[idx][1] = r * rUA / rp
			a.lensTransformData[idx][2] = float32(math.Cos(angleRad))
		}
	}
}

func (a *App) getAngle(x float32, y float32) float32 {
	radius := SENSOR_PIXEL_REAL_SIZE * float32(math.Sqrt(float64((x*x)+(y*y))))
	alfaGrad := float32(0)
	for i := 1; i < CameraLensSize; i++ {
		if radius >= RealImageHeight[i-1] && radius <= RealImageHeight[i] {
			alfaGrad = a.interpolate(radius, RealImageHeight[i-1], Angle[i-1], RealImageHeight[i], Angle[i])
		}
	}
	return alfaGrad
}

func (a *App) interpolate(xIn float32, x0 float32, y0 float32, x1 float32, y1 float32) float32 {
	return ((xIn - x0) * (y1 - y0) / (x1 - x0)) + y0
}

func (a *App) calcPointCloud(distance uint16, index int) Field3D {
	distanceF := float32(distance)
	distortData := a.lensTransformData[index]
	return [4]float32{
		distanceF * distortData[0],
		distanceF * distortData[1],
		distanceF * distortData[2],
		0,
	}
}

func (a *App) cbServo(_ *sc.SerialConnection, data []byte) {
	a.currentDegrees = float64(binary.LittleEndian.Uint16(data))
	a.degreeChange = time.Now().UnixMilli()
	a.needRecord = true
	runtime.LogDebugf(a.ctx, "Servo at: %.2f", a.currentDegrees)
	runtime.EventsEmit(a.ctx, "direction", fmt.Sprintf("%.0f", a.currentDegrees))
}

func (a *App) cbSensor(_ *sc.SerialConnection, data []byte) {
	if !a.needRecord || time.Now().UnixMilli()-a.degreeChange < 500 || a.degreeChange == 0 {
		return
	}
	degree := a.currentDegrees
	intDegree := uint16(degree)
	for row := 0; row < SENSOR_ROW; row++ {
		for col := 0; col < SENSOR_COL; col++ {
			bufferIndex := col + (row * SENSOR_COL)
			byteStart := uint16(float32(bufferIndex*12) / float32(8))

			val := uint16(0)
			if bufferIndex%2 == 0 {
				val = uint16(data[byteStart]) << 4
				val = val + ((uint16(data[byteStart+1]) & 0xF0) >> 4)
			} else {
				val = uint16(data[byteStart]&0x0F) << 8
				val = val + (uint16(data[byteStart+1]))
			}

			a.scanResult[intDegree][row][col][3] = float32(val)

			if val >= SENSOR_MAX_DEPTH_3D {
				a.scanResult[intDegree][row][col][0] = 0
				a.scanResult[intDegree][row][col][1] = 0
				a.scanResult[intDegree][row][col][2] = 0
				continue
			}

			cloudData := a.calcPointCloud(val, bufferIndex)
			a.scanResult[intDegree][row][col][0] = cloudData[2] * MM2M
			a.scanResult[intDegree][row][col][1] = -cloudData[0] * MM2M
			a.scanResult[intDegree][row][col][2] = -cloudData[1] * MM2M
		}
	}
	a.needRecord = false
	runtime.LogDebugf(a.ctx, "Took snapshot of %.0f°", degree)

	// Dump for image generation (2D)
	if intDegree%40 == 0 || a.isFixRoation {
		a.lastWrite = time.Now().Unix()
		_ = a.toPCD(int(intDegree))
	}

	if intDegree == 180 && !a.isFixRoation {
		a.lastWriteRoom = time.Now().Unix()
		_ = a.toPCD(-1)
	}
}

func (a *App) toPCD(onlyDegree int) error {
	filename := fmt.Sprintf("sensor_%d.pcd", onlyDegree)
	if onlyDegree == -1 {
		filename = "sensor_room.pcd"
	}
	pcd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer pcd.Close()

	var dat *os.File
	if onlyDegree != -1 {
		dat, err = os.OpenFile("sensor.dat", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}
		defer dat.Close()
	}

	var degreeSet []int
	size := 0
	for degree := range a.scanResult {
		if onlyDegree != -1 && degree != onlyDegree {
			continue
		}
		oldSize := size
		for row := 0; row < SENSOR_ROW; row++ {
			for col := 0; col < SENSOR_COL; col++ {
				val := a.scanResult[degree][row][col]
				x := val[0]
				y := val[1]
				z := val[2]

				if x == 0 && y == 0 && z == 0 {
					continue
				}
				size++
			}
		}
		if oldSize != size {
			degreeSet = append(degreeSet, degree)
		}
	}

	_, _ = pcd.WriteString("VERSION .7\n")
	_, _ = pcd.WriteString("FIELDS x y z\n")
	_, _ = pcd.WriteString("SIZE 4 4 4\n")
	_, _ = pcd.WriteString("TYPE F F F\n")
	_, _ = pcd.WriteString("COUNT 1 1 1\n")
	_, _ = pcd.WriteString(fmt.Sprintf("WIDTH %d\n", size))
	_, _ = pcd.WriteString("HEIGHT 1\n")
	_, _ = pcd.WriteString("VIEWPOINT 0 0 0 1 0 0 0\n")
	_, _ = pcd.WriteString(fmt.Sprintf("POINTS %d\n", size))
	_, _ = pcd.WriteString("DATA ascii\n")

	for _, degree := range degreeSet {
		for row := 0; row < SENSOR_ROW; row++ {
			for col := 0; col < SENSOR_COL; col++ {
				val := a.scanResult[degree][row][col]
				// https://de.mathworks.com/matlabcentral/answers/123763-how-to-rotate-entire-3d-data-with-x-y-z-values-along-a-particular-axis-say-x-axis
				x := val[0]
				y := val[1]
				z := val[2]

				if dat != nil {
					_, _ = dat.WriteString(fmt.Sprintf("%.0f ", val[3]))
				}

				if x == 0 && y == 0 && z == 0 {
					continue
				}

				//angleRad := float64(degree) * math.Pi / 180.0

				//roatedX := x*float32(math.Cos(angleRad)) + z*float32(math.Sin(angleRad))
				//roatedY := y*float32(math.Sin(float64(degree))) + y*float32(math.Cos(float64(degree)))
				//roatedZ := z*float32(math.Cos(angleRad)) + x*float32(math.Sin(angleRad))

				_, _ = pcd.WriteString(fmt.Sprintf("%.5f %.5f %.5f\n", x, y, z))
			}
			if dat != nil {
				_, _ = dat.WriteString("\n")
			}
		}
	}

	if onlyDegree != -1 {
		_, _ = pcd.Seek(0, io.SeekStart)
		b := new(strings.Builder)
		_, _ = io.Copy(b, pcd)
		runtime.EventsEmit(a.ctx, "data", b.String())
	}

	runtime.LogInfo(a.ctx, "Wrote PCD file")
	return nil
}

func (a *App) GetPorts() string {
	ports, err := serial.GetPortsList()
	if err != nil {
		runtime.LogErrorf(a.ctx, "Failed get ports %v", err)
		return ""
	}
	parsedAsJSON, err := json.Marshal(ports)
	if err != nil {
		runtime.LogErrorf(a.ctx, "Failed to generate JSON %v", err)
		return ""
	}
	return string(parsedAsJSON)
}

// DOMReady is called after the front-end dom has been loaded
func (a *App) DOMReady(ctx context.Context) {
	runtime.EventsOn(ctx, "start", func(optionalData ...interface{}) {
		if len(optionalData) == 0 {
			return
		}
		data, ok := optionalData[0].(map[string]interface{})
		if !ok {
			runtime.LogError(ctx, "Failed to parse optionalData on start")
			return
		}

		parsedAsJSON, err := json.Marshal(data)
		if err != nil {
			runtime.LogErrorf(ctx, "Failed to generate JSON %v", err)
			return
		}

		reqData := struct {
			ServoPort   string `json:"servoPort"`
			SensorPort  string `json:"sensorPort"`
			FixRotation int16  `json:"fixRotation"`
		}{}
		if err := json.Unmarshal(parsedAsJSON, &reqData); err != nil {
			runtime.LogErrorf(ctx, "Failed to parse generated JSON %v", err)
			return
		}
		a.servo = sc.NewSerialConnection(a.ctx, reqData.ServoPort, sc.TYPE_SERVO, a.cbServo)
		a.sensor = sc.NewSerialConnection(a.ctx, reqData.SensorPort, sc.TYPE_SENSOR, a.cbSensor)

		err = a.servo.Start(reqData.FixRotation)
		if err != nil {
			runtime.LogErrorf(a.ctx, "Failed to start servo serial port %v", err)
		}

		err = a.sensor.Start(-1)
		if err != nil {
			_ = a.servo.Stop()
			runtime.LogErrorf(a.ctx, "Failed to start sensor serial port %v", err)
		}

		a.isFixRoation = reqData.FixRotation >= 0
	})

	runtime.EventsOn(ctx, "stop", func(optionalData ...interface{}) {
		if a.servo != nil {
			_ = a.servo.Stop()
		}

		if a.sensor != nil {
			_ = a.sensor.Stop()
		}

		runtime.EventsEmit(a.ctx, "stopped")
	})

	runtime.EventsOn(ctx, "notification", func(optionalData ...interface{}) {
		if len(optionalData) == 0 {
			return
		}
		data, ok := optionalData[0].(map[string]interface{})
		if !ok {
			runtime.LogError(ctx, "Failed to parse optionalData on notification")
			return
		}

		parsedAsJSON, err := json.Marshal(data)
		if err != nil {
			runtime.LogErrorf(ctx, "Failed to generate JSON %v", err)
			return
		}

		reqData := struct {
			DialogType string `json:"type"`
			Title      string `json:"title"`
			Message    string `json:"message"`
		}{}
		if err := json.Unmarshal(parsedAsJSON, &reqData); err != nil {
			runtime.LogErrorf(ctx, "Failed to parse generated JSON %v", err)
			return
		}

		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.DialogType(reqData.DialogType),
			Title:   reqData.Title,
			Message: reqData.Message,
		})
	})

}
