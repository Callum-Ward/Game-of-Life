// Used for each worker to get registered by the broker

package stubsWorkerToBroker

var HandleWorker = "GameOfLife.RegisterWorker"

type Response struct {
}

type Request struct {
	SocketAddress string
}
