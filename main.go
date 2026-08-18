package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	faceFront = iota
	faceRight
	faceLeft
	faceBack
	faceBottom
	faceTop
)

type CubeRenderer struct {
	width, height int

	zBuffer []float64
	buffer  []byte

	angleX, angleY, angleZ float64

	cubeWidth       float64
	distanceFromCam float64
	zoomLevel       float64
	out             *bufio.Writer
}

func NewCubeRenderer(w, h int) *CubeRenderer {
	return &CubeRenderer{
		width: w, height: h,
		zBuffer: make([]float64, w*h),
		buffer:  make([]byte, w*h),
		angleX:  0, angleY: 0, angleZ: 0,
		cubeWidth:       20,
		distanceFromCam: 100,
		zoomLevel:       30,
		out:             bufio.NewWriter(os.Stdout),
	}
}

func (cr *CubeRenderer) CalculateX(i, j, k float64) float64 {
	return i*math.Cos(cr.angleZ)*math.Cos(cr.angleY) + j*(math.Cos(cr.angleZ)*math.Sin(cr.angleY)*math.Sin(cr.angleX)-math.Sin(cr.angleZ)*math.Cos(cr.angleX)) + k*(math.Cos(cr.angleZ)*math.Sin(cr.angleY)*math.Cos(cr.angleX)+math.Sin(cr.angleZ)*math.Sin(cr.angleX))
}

func (cr *CubeRenderer) CalculateY(i, j, k float64) float64 {
	return i*math.Sin(cr.angleZ)*math.Cos(cr.angleY) + j*(math.Sin(cr.angleZ)*math.Sin(cr.angleY)*math.Sin(cr.angleX)+math.Cos(cr.angleZ)*math.Cos(cr.angleX)) + k*(math.Sin(cr.angleZ)*math.Sin(cr.angleY)*math.Cos(cr.angleX)-math.Cos(cr.angleZ)*math.Sin(cr.angleX))
}

func (cr *CubeRenderer) CalculateZ(i, j, k float64) float64 {
	return i*-math.Sin(cr.angleY) + j*math.Cos(cr.angleY)*math.Sin(cr.angleX) + k*math.Cos(cr.angleY)*math.Cos(cr.angleX)
}

func ClearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to clear screen: %v\n", err)
	}
}

func HideCursor() {
	fmt.Print("\033[?25l")
}

func ShowCursor() {
	fmt.Print("\033[?25h")
}

func (cr *CubeRenderer) DrawPoint(x, y, z float64, character byte) {
	xp := cr.CalculateX(x, y, z)
	yp := cr.CalculateY(x, y, z)
	zp := cr.CalculateZ(x, y, z) + float64(cr.distanceFromCam)

	// Check if it behind , if behind than we skip drawing this point
	if zp <= 0 {
		return
	}

	// Project it to 3d
	oneOverZ := 1 / zp
	screenX := int(float64(cr.width)/2 + cr.zoomLevel*oneOverZ*xp*2)
	screenY := int(float64(cr.height)/2 + cr.zoomLevel*oneOverZ*yp*2)

	// Check the screen bound
	if screenX < 0 || screenX >= cr.width || screenY < 0 || screenY >= cr.height {
		return
	}

	// Converts 3d screen coords to 1D for buffers
	index := screenY*cr.width + screenX

	// zBuffer[index] = 1/z of the nearest point drawn here so far (bigger = closer).
	if oneOverZ > cr.zBuffer[index] {
		cr.zBuffer[index] = oneOverZ // New closer point
		cr.buffer[index] = character // Update visible pixel to match
	}
}

func (cr *CubeRenderer) RenderCube() {
	increment := 0.8 // density of each side

	chars := []byte{'@', '#', '%', '*', '&', '$'}

	// Render the character each side of the cube

	for x := -cr.cubeWidth; x < cr.cubeWidth; x += increment {
		for y := -cr.cubeWidth; y < cr.cubeWidth; y += increment {
			cr.DrawPoint(x, y, -cr.cubeWidth, chars[faceFront])
			cr.DrawPoint(cr.cubeWidth, y, x, chars[faceRight])
			cr.DrawPoint(-cr.cubeWidth, y, -x, chars[faceLeft])
			cr.DrawPoint(-x, y, cr.cubeWidth, chars[faceBack])
			cr.DrawPoint(x, -cr.cubeWidth, -y, chars[faceBottom])
			cr.DrawPoint(x, cr.cubeWidth, y, chars[faceTop])
		}
	}
}

func (cr *CubeRenderer) Display() {
	var frame strings.Builder

	// Pre-allocate the frame
	frame.Grow(cr.width*cr.height + cr.height)

	// Move cursor  home
	frame.WriteString("\x1b[H")

	// Build a frame
	for y := 0; y < cr.height; y++ {
		start := y * cr.width

		frame.Write(cr.buffer[start : start+cr.width])

		frame.WriteByte('\n')
	}

	// Write to the buffered writer
	if _, err := cr.out.WriteString(frame.String()); err != nil {
		fmt.Fprintf(os.Stderr, "Fail to Write: %v", err)
		return
	}
}

func (cr *CubeRenderer) Rotate() {
	cr.angleX += 0.05
	cr.angleY += 0.05
	cr.angleZ += 0.01
}

func (cr *CubeRenderer) Run() {
	// Setup the terminal
	ClearScreen() // clear terminal
	HideCursor()  // hide terminal cursor

	// Show cursor after stop the program
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	cleanup := func() {
		ShowCursor()
		if err := cr.out.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Flush error: %v\n", err)
		}
	}

	go func() {
		<-sigCh
		cleanup()
		os.Exit(0)
	}()

	//  In case we do the if statement to quick the program
	defer cleanup()

	for {
		// reset buffer, ensuring each fram starts clean
		for i := range cr.buffer {
			cr.buffer[i] = ' '
		}
		for i := range cr.zBuffer {
			cr.zBuffer[i] = 0
		}

		// Render cube
		cr.RenderCube()

		// Display and rotate
		cr.Display()
		cr.Rotate()

		// wait
		time.Sleep(16 * time.Millisecond)
	}
}

func main() {
	cube := NewCubeRenderer(140, 40)

	cube.Run()
}
