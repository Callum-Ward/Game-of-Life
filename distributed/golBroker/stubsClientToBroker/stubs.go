package stubsClientToBroker

var ProcessWorldHandler = "GameOfLife.ProcessWorld"
var ProcessTimerEventsHandler = "GameOfLife.ProcessAliveCellsCount"

type Response struct {
	ProcessedWorld [][]uint8
}

type Request struct {
	WorldSection [][]uint8
	ImageHeight  int
	ImageWidth   int
	Turns        int
}

type ResponseToAliveCellsCount struct {
	AliveCellsCount int
	Turn int
}

type RequestAliveCellsCount struct {
	ImageHeight int
	ImageWidth  int
}
