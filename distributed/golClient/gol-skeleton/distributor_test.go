package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

// Benchmark applies the filter to the 512x512 image b.N times.
// The time taken is carefully measured by go.
// The b.N repetition is needed because benchmark results are not always constant.
func BenchmarkFilter(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	var params gol.Params
	params.ImageWidth = 5120
	params.ImageHeight = 5120
	params.Turns = 30
	// For-loop to run 5 sub-benchmarks, with 1, 2, 4, 8 and 16 workers.
	//for threads := 1; threads <= 512; threads*=2 {
		fmt.Println("running with", 1, "threads")
		b.Run(fmt.Sprintf("%d_workers", 1), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				params.Threads = 1
				keyPresses := make(chan rune, 10)
				events := make(chan gol.Event, 1000)
				go gol.Run(params, events, keyPresses)
				complete := false
				for !complete {
					event := <-events
					switch event.(type) {
					case gol.FinalTurnComplete:
						complete = true
					}
				}
			}
		})
	//}
}
