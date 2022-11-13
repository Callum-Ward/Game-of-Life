package stubsKeyPresses

var KeyPressHandler = "GameOfLife.ProcessKeyPresses"

type ResponseToKeyPress struct {

	WorldSection [][]uint8 	//case s world is returned
	Turn int //case p need to print turn
}

type RequestFromKeyPress struct {
	KeyPressed string
}
