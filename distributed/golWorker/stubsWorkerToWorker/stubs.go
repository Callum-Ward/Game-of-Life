package stubsWorkerToWorker

var ProcessRowExchange = "GameOfLife.ProcessRowExchange"


type RequestRow struct {
	Turn int
	Row []uint8
}

type ResponseRow struct {
	Row []uint8
}




