package main

import (
	"flag"
	"fmt"
	"golDist/stubsBrokerToWorker"
	"golDist/stubsClientToBroker"
	"golDist/stubsKeyPresses"
	"golDist/stubsWorkerToBroker"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"time"
)

var shutdown bool
var workerAddresses []string
var workers []worker
var ImageHeight int
var ImageWidth int
type GameOfLife struct{}

type worker struct {
	client *rpc.Client
	ImageHeight int
	ImageWidth int
}

func makeMatrix(height, width int) [][]uint8 {
	matrix := make([][]uint8, height)
	for i := range matrix {
		matrix[i] = make([]uint8, width)
	}
	return matrix
}

func makeWorkerSlice(world [][]uint8, mainBlockLen,curBlockLen,blockNo int) [][]uint8 {
	worldSection := makeMatrix(curBlockLen+2, ImageWidth)
	for x:=mainBlockLen*blockNo;x<blockNo*mainBlockLen+curBlockLen+2;x++{
		worldSection[x-mainBlockLen*blockNo] = world[(x-1+ImageHeight) % ImageHeight]
	}
	return worldSection
}

func runWorker(WorkerSocket,BottomSocket string,section [][]uint8,height,turns int, finishedSection chan<- [][]uint8, interrupt chan<- bool) {
	fmt.Println("Worker: " + WorkerSocket)
	client, err := rpc.Dial("tcp", WorkerSocket)
	workers = append(workers,worker{client: client,ImageHeight:height,ImageWidth:len(section[0])})
	if err != nil {panic(err)}
	defer client.Close()
	response := new(stubsBrokerToWorker.Response)
	//ImageHeight passed includes the halos
	request := stubsBrokerToWorker.Request{WorldSection:section,ImageHeight:height,ImageWidth:len(section[0]) ,Turns: turns,BottomSocketAddress: BottomSocket}

	err = client.Call(stubsBrokerToWorker.ProcessWorldHandler, request, response)
	if err != nil {panic(err)}
	finishedSection <- response.ProcessedSection
}

func (s *GameOfLife) RegisterWorker(req stubsWorkerToBroker.Request, res *stubsWorkerToBroker.Response) (err error) {
	fmt.Println("registering a worker")
	workerAddresses = append(workerAddresses, req.SocketAddress)
	res = nil
	return
}

func (s *GameOfLife) ProcessKeyPresses(req stubsKeyPresses.RequestFromKeyPress, res *stubsKeyPresses.ResponseToKeyPress) (err error) {
	var currentWorld [][]uint8
	if req.KeyPressed == "s" || req.KeyPressed == "k" {currentWorld = makeMatrix(ImageHeight,ImageWidth)}
	for _,worker := range workers {
		worker.client.Call(stubsKeyPresses.KeyPressHandler,req,res)
		if req.KeyPressed == "s" || req.KeyPressed == "k" {currentWorld = append(currentWorld, res.WorldSection...)}
	}
	if req.KeyPressed == "s" || req.KeyPressed == "k" {res.WorldSection = currentWorld}
	if req.KeyPressed == "k" {shutdown = true}
	return
}

func (s *GameOfLife) ProcessAliveCellsCount(req stubsClientToBroker.RequestAliveCellsCount , res *stubsClientToBroker.ResponseToAliveCellsCount) (err error) {
	totalAliveCells := 0
	turnA := 0
	turnB := 0
	for i,worker := range workers {
		response := new(stubsBrokerToWorker.ResponseToAliveCellsCount)
		request := stubsBrokerToWorker.RequestAliveCellsCount{ImageHeight:worker.ImageHeight, ImageWidth:worker.ImageWidth}
		if i != 0{
			request = stubsBrokerToWorker.RequestAliveCellsCount{Turn: turnA,ImageHeight:worker.ImageHeight, ImageWidth:worker.ImageWidth}
		}
		worker.client.Call(stubsBrokerToWorker.ProcessTimerEventsHandler,request,response)
		totalAliveCells += response.AliveCellsCount
		if i == 0 { turnA = response.Turn}
		if i == 1 { turnB = response.Turn}
	}
	if turnA != turnB {fmt.Println("mismatched turns")}
	res.Turn = turnA
	fmt.Println("alive cells is", totalAliveCells, "at turn", res.Turn)
	res.AliveCellsCount = totalAliveCells
	return
}

func (s *GameOfLife) ProcessWorld(req stubsClientToBroker.Request, res *stubsClientToBroker.Response) (err error) {
	blockCount := 0
	ImageHeight = req.ImageHeight
	ImageWidth = req.ImageWidth
	workers := len(workerAddresses)
	mainBlockLen := int(math.Floor(float64(req.ImageHeight) / float64(workers)))
	outChannels := make([]chan [][]uint8, 0)
	interrupt := make(chan bool)

	if workers > 0 && workers <= req.ImageHeight  {
		for yPos := 0; yPos <= req.ImageHeight-mainBlockLen; yPos += mainBlockLen {
			BottomSocket := workerAddresses[(blockCount+workers+1)%workers]
			worldSection := makeWorkerSlice(req.WorldSection,mainBlockLen,mainBlockLen,blockCount)
			outChannels = append(outChannels, make(chan [][]uint8))
			go runWorker(workerAddresses[blockCount],BottomSocket,worldSection,mainBlockLen+2,req.Turns,outChannels[blockCount], interrupt)
			blockCount++
			if blockCount == workers-1 && req.ImageHeight-(yPos+mainBlockLen) > mainBlockLen {break}
		}
		if blockCount != workers {
			fmt.Println("giving bigger slice to last worker")
			BottomSocket := workerAddresses[0]
			bigSliceHeight := ImageHeight-(blockCount*mainBlockLen)+2
			worldSection := makeWorkerSlice(req.WorldSection,mainBlockLen,bigSliceHeight,blockCount)
			outChannels = append(outChannels, make(chan [][]uint8))
			go runWorker(workerAddresses[blockCount],BottomSocket,worldSection,bigSliceHeight,req.Turns,outChannels[blockCount], interrupt)
		}
		finishedWorld := make([][]uint8, 0)
		for block := 0; block < workers; block++ {
			finishedWorld = append(finishedWorld, <-outChannels[block]...)
		}

		res.ProcessedWorld = finishedWorld
	} else {panic("No workers available")}

	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLife{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	go rpc.Accept(listener)
	for {
		if shutdown {
			time.Sleep(time.Second * 1)
			break
		}
	}
}
