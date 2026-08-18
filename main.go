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

type CubeRenderer struct {
	width, height int

	zBuffer []float64
	buffer  []byte

	A, B, C float64

	cubeWidth       float64
	distanceFromCam float64
	zoomlevel       float64
	out             *bufio.Writer
}

func NewCubeRenderer(w, h int) *CubeRenderer {
	return &CubeRenderer{
		width: w, height: h,
		zBuffer: make([]float64, w*h),
		buffer:  make([]byte, w*h),
		A:       0, B: 0, C: 0,
		cubeWidth:       20,
		distanceFromCam: 100,
		zoomlevel:       30,
		out:             bufio.NewWriter(os.Stdout),
	}
}

func (cr *CubeRenderer) CalculateX(i, j, k float64) float64 {
	return i*math.Cos(cr.C)*math.Cos(cr.B) + j*(math.Cos(cr.C)*math.Sin(cr.B)*math.Sin(cr.A)-math.Sin(cr.C)*math.Cos(cr.A)) + k*(math.Cos(cr.C)*math.Sin(cr.B)*math.Cos(cr.A)+math.Sin(cr.C)*math.Sin(cr.A))
}

func (cr *CubeRenderer) CalculateY(i, j, k float64) float64 {
	return i*math.Sin(cr.C)*math.Cos(cr.B) + j*(math.Sin(cr.C)*math.Sin(cr.B)*math.Sin(cr.A)+math.Cos(cr.C)*math.Cos(cr.A)) + k*(math.Sin(cr.C)*math.Sin(cr.B)*math.Cos(cr.A)-math.Cos(cr.C)*math.Sin(cr.A))
}

func (cr *CubeRenderer) CalculateZ(i, j, k float64) float64 {
	return i*-math.Sin(cr.B) + j*math.Cos(cr.B)*math.Sin(cr.A) + k*math.Cos(cr.B)*math.Cos(cr.A)
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
	oneoverz := 1 / zp
	screenx := int(float64(cr.width)/2 + cr.zoomlevel*oneoverz*xp*2)
	screeny := int(float64(cr.height)/2 + cr.zoomlevel*oneoverz*yp*2)

	// Check the screen bound
	if screenx < 0 || screenx >= cr.width || screeny < 0 || screeny >= cr.height {
		return
	}

	// Converts 3d screen coords to 1D for buffers
	index := screeny*cr.width + screenx

	// zBuffer[index] = 1/z of the nearest point drawn here so far (bigger = closer).
	if oneoverz > cr.zBuffer[index] {
		cr.zBuffer[index] = oneoverz // New closer point
		cr.buffer[index] = character // Update visible pixel to match
	}
}

func (cr *CubeRenderer) RenderCube() {
	incrrement := 0.8 // density of each side

	chars := []byte{'@', '#', '%', '.', '=', '^'}

	for x := -cr.cubeWidth; x < cr.cubeWidth; x += incrrement {
		for y := -cr.cubeWidth; y < cr.cubeWidth; y += incrrement {
			// front side
			cr.DrawPoint(x, y, -cr.cubeWidth, chars[0])
			// right side
			cr.DrawPoint(cr.cubeWidth, y, x, chars[1])
			// left side
			cr.DrawPoint(-cr.cubeWidth, y, -x, chars[2])
			// back side
			cr.DrawPoint(-x, y, cr.cubeWidth, chars[3])
			// bottom side
			cr.DrawPoint(x, -cr.cubeWidth, -y, chars[4])
			// top side
			cr.DrawPoint(x, cr.cubeWidth, y, chars[5])
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
	cr.A += 0.05
	cr.B += 0.05
	cr.C += 0.01
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

	//  incase we do the if statement to quick the program
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
