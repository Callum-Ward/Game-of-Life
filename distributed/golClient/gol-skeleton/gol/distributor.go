package gol

import "C"
import (
	"fmt"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubsClientToServer"
	"uk.ac.bris.cs/gameoflife/stubsKeyPresses"
	"uk.ac.bris.cs/gameoflife/util"
)

var lastKeyPressed string

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}


func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func outputBoard(world [][]byte, p Params, c distributorChannels) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
}

func processKeyPresses(client *rpc.Client , keyPresses <-chan rune,done <-chan bool, p Params, c distributorChannels) {
	for {
		select {
		case <-done:
			return
		case key := <-keyPresses:
			response := new(stubsKeyPresses.ResponseToKeyPress)
			request := new(stubsKeyPresses.RequestFromKeyPress)
			switch key {
			case 112: // P: pause processing and print current turn, if p pressed again then resume
				request.KeyPressed = "p"
				if lastKeyPressed == "p" {
					fmt.Println("Continuing")
					lastKeyPressed = ""
				} else {lastKeyPressed = "p"}

			case 113: // Q: Generate PGM file with current state of board and terminate
				request.KeyPressed = "q"
				lastKeyPressed = "q"
				fmt.Println("q pressed")

			case 115: // S: Generate PGM file with current state of board
				request.KeyPressed = "s"
				fmt.Println("s pressed")

			case 107: // K Generate PGM file and shutdown server
				request.KeyPressed = "k"
				fmt.Println("k pressed")
			}
			client.Call(stubsKeyPresses.KeyPressHandler,request,response)

			if lastKeyPressed == "p" {fmt.Println(fmt.Sprintf("Paused on turn: %d",response.Turn))}
			if key == 115 {outputBoard(response.WorldSection,p,c)}
		}
	}
}

func getLiveCells(p Params, oWorld [][]uint8) []util.Cell {
	liveCells := make([]util.Cell, 0)
	for x := 0; x < p.ImageWidth; x++ {
		for y := 0; y < p.ImageHeight; y++ {
			if oWorld[y][x] == 255 {
				var currentCell = util.Cell{X: x, Y: y}
				liveCells = append(liveCells, currentCell)
			}
		}
	}
	return liveCells
}

func tickerFunc(done chan bool, ticker time.Ticker, client *rpc.Client, p Params, c distributorChannels) {
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			response := new(stubsClientToServer.ResponseToAliveCellsCount)
			request := stubsClientToServer.RequestAliveCellsCount{ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth}
			err := client.Call(stubsClientToServer.ProcessTimerEventsHandler, request, response)
			if err != nil {
				fmt.Println(err.Error())
			}
			c.events <- AliveCellsCount{CompletedTurns: response.Turn, CellsCount: response.AliveCellsCount}
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	oWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			oWorld[y][x] = <-c.ioInput
		}
	}
	server := "127.0.0.1:8030"
	fmt.Println("Server: " + server)
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		fmt.Println(err.Error())
	}
	defer client.Close()
	done := make(chan bool)
	ticker := time.NewTicker(2 * time.Second)
	response := new(stubsClientToServer.Response)
	request := stubsClientToServer.Request{WorldSection: oWorld, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth, Turns: p.Turns}
	go processKeyPresses(client, keyPresses,done,p,c)
	go tickerFunc(done, *ticker, client, p, c)
	client.Call(stubsClientToServer.ProcessWorldHandler, request, response)
	ticker.Stop()
	done <- true
	done <- true
	cellsAlive := getLiveCells(p, response.ProcessedWorld)
	// Make sure that the Io has finished any output before exiting.
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: cellsAlive}
	if lastKeyPressed != "q" {outputBoard(response.ProcessedWorld,p,c)}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- StateChange{p.Turns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
