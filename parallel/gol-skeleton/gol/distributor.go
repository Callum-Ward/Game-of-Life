package gol

import "C"
import (
	"fmt"
	"math"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// makeMatrix creates a new matrix of specified height and width
func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

// makeImmutableMatrix returns a function which in turn returns an immutable matrix
func makeImmutableMatrix(matrix [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return matrix[y][x]
	}
}

// outputBoard sends a board to io for output
func outputBoard(world [][]byte, p Params, c distributorChannels) {
	c.ioCommand <- ioOutput
	c.ioFilename <- fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
}

// performTurn uses Game of Life rules to advance game state
func performTurn(world func(y, x int) byte, newWorld [][]byte, p Params, c distributorChannels, startY, endY, startX, endX int) {
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			aliveCells := 0
			aliveCells += int(world((y+p.ImageHeight-1)%p.ImageHeight, (x+p.ImageWidth-1)%p.ImageWidth)) //top left
			aliveCells += int(world((y+p.ImageHeight-1)%p.ImageHeight, x)) //top middle
			aliveCells += int(world((y+p.ImageHeight-1)%p.ImageHeight, (x+p.ImageWidth+1)%(p.ImageWidth))) //top right
			aliveCells += int(world(y, (x+p.ImageWidth-1)%p.ImageWidth)) //middle left
			aliveCells += int(world(y, (x+p.ImageWidth+1)%p.ImageWidth)) //middle right
			aliveCells += int(world((y+p.ImageHeight+1)%p.ImageHeight, (x+p.ImageWidth-1)%p.ImageWidth)) //bottom left
			aliveCells += int(world((y+p.ImageHeight+1)%p.ImageHeight, x))  //bottom middle
			aliveCells += int(world((y+p.ImageHeight+1)%p.ImageHeight, (x+p.ImageWidth+1)%p.ImageWidth)) //bottom right
			if aliveCells > 0 {aliveCells = aliveCells / 255}
			if world(y, x) == 255 {
				if aliveCells < 2 || aliveCells > 3 {
					newWorld[y][x] = 0
					c.events <- CellFlipped{CompletedTurns: 1, Cell: util.Cell{X: x, Y: y}}
				}
			} else {
				if aliveCells == 3 {
					newWorld[y][x] = 255
					c.events <- CellFlipped{CompletedTurns: 1, Cell: util.Cell{X: x, Y: y}}
				}
			}
		}
	}
}

// worker calls performTurn on required data and sends signal to out when processing complete
func worker(startY, endY, startX, endX int, p Params, c distributorChannels, data func(y, x int) byte, newWorld [][]byte, out chan<- bool) {
	performTurn(data, newWorld, p, c, startY, endY, startX, endX)
	out <- true
}

// tickerfunc sends an alive cells count event every two seconds
func tickerfunc(done chan bool, ticker time.Ticker, c distributorChannels, p Params, oWorld *[][]uint8, turn *int) {
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			liveCells := 0
			for y := 0; y < p.ImageWidth; y++ {
				for x := 0; x < p.ImageHeight; x++ {
					if (*oWorld)[y][x] == 255 {
						liveCells++
					}
				}
			}
			c.events <- AliveCellsCount{CompletedTurns: *turn, CellsCount: liveCells}
		}
	}
}

// processKeyPresses constantly checks for keyboard input and sends interrupts accordingly
func processKeyPresses(world [][]byte, turn *int, p Params, c distributorChannels, keyPresses <-chan rune, qinterrupt chan<- bool, pauseinterrupt chan<-bool, resumeinterrupt chan<-bool) {
	for {
		select {
		case key := <-keyPresses:
			switch key {
			case 112: // P: pause processing and print current turn, if p pressed again then resume
				fmt.Println("Current turn is:", *turn)
				pauseinterrupt <- true
				for {
					if <-keyPresses == 112 {
						resumeinterrupt <- true
						fmt.Println("Continuing")
						break
					}
				}
			case 113: // Q: Generate PGM file with current state of board and terminate
				fmt.Println("q pressed")
				qinterrupt <- true
			case 115: // S: Generate PGM file with current state of board
				outputBoard(world, p, c)
			}
		}
	}
}

// getLiveCells is a helper function to return an array of live cells
func getLiveCells(p Params, oWorld [][]uint8) []util.Cell{
	liveCells := make([]util.Cell, 0)
	for y := 0; y < p.ImageWidth; y++ {
		for x := 0; x < p.ImageHeight; x++ {
			if oWorld[y][x] == 255 {
				var currentCell = util.Cell{X: x, Y: y}
				liveCells = append(liveCells, currentCell)
			}
		}
	}
	return liveCells
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {
	// Get input from file
	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	oWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
	cpyWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			oWorld[y][x] = <-c.ioInput
			cpyWorld[y][x] = oWorld[y][x]
			if oWorld[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: 1, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	// Set up channels and vars used in processing game
	turn := 0
	interrupt := false
	done := make(chan bool)
	out := make(chan bool)
	qinterrupt := make(chan bool)
	pause := make(chan bool)
	resume := make(chan bool)
	ticker := time.NewTicker(2 * time.Second)
	go tickerfunc(done, *ticker, c, p, &oWorld, &turn)
	go processKeyPresses(cpyWorld, &turn, p, c, keyPresses, qinterrupt, pause, resume)
	blockLen := int(math.Floor(float64(p.ImageHeight) / float64(p.Threads)))

	// Process all turns of game
	for turn < p.Turns {
		select{
		case <- qinterrupt:
			interrupt = true
			outputBoard(oWorld, p, c)
			c.ioCommand <- ioCheckIdle
			<-c.ioIdle
			turn = p.Turns
			break
		case <-pause:
			<-resume
			break
		default:
			// Create new world for workers to populate
			cpyWorld := makeMatrix(p.ImageHeight, p.ImageWidth)
			for y := 0; y < p.ImageWidth; y++ {
				for x := 0; x < p.ImageHeight; x++ {
					cpyWorld[y][x] = oWorld[y][x]
				}
			}
			immutableData := makeImmutableMatrix(oWorld)

			// Instantiate worker threads
			if p.Threads <= p.ImageHeight {
				blockCount := 0
				for yPos := 0; yPos <= p.ImageHeight-blockLen; yPos += blockLen {
					go worker(yPos, yPos+blockLen, 0, p.ImageWidth, p, c, immutableData, cpyWorld, out)
					blockCount++
					if blockCount == p.Threads-1 && p.ImageHeight-(yPos+blockLen) > blockLen {break}
				}
				if blockCount != p.Threads {
					go worker(blockCount*blockLen, p.ImageHeight, 0, p.ImageWidth, p, c, immutableData, cpyWorld, out)
					blockCount++
				}
				for block := 0; block < p.Threads; block++ {
					<-out
				}
			}

			// Complete turn with new data
			oWorld = cpyWorld
			turn++
			c.events <- TurnComplete{CompletedTurns: turn}
		}
	}

	// Calculate which cells are live
	c.events <- FinalTurnComplete{turn, getLiveCells(p, oWorld)}

	// Output the state of the board if program finished (rather than interrupt)
	if !interrupt {
		outputBoard(oWorld, p, c)
	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// Shut down procedure
	ticker.Stop()
	done <- true
	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)

}
