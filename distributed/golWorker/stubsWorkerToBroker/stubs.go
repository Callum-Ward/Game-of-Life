package stubsWorkerToBroker

var HandleWorker = "GameOfLife.RegisterWorker"

type Response struct {
}

type Request struct {
	SocketAddress string
}
