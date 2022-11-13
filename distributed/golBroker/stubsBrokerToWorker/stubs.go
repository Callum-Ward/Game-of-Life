// Used

package stubsBrokerToWorker

var ProcessWorldHandler = "GameOfLife.ProcessWorld"
var ProcessTimerEventsHandler = "GameOfLife.ProcessAliveCellsCount"

type Response struct {
	ProcessedSection [][]uint8
}

type Request struct {
	WorldSection [][]uint8
	ImageHeight  int
	ImageWidth   int
	Turns        int
	BottomSocketAddress string
}

type ResponseToAliveCellsCount struct {
	AliveCellsCount int
	Turn int
}

type RequestAliveCellsCount struct {
	Turn int
	ImageHeight int
	ImageWidth  int
}
