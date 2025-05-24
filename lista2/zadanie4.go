package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
	"unicode"
)

const (
	NrOfTravelers      = 15
	NrOfWildTenants    = 10
	MinSteps           = 10
	MaxSteps           = 100
	MinDelay           = 10 * time.Millisecond
	MaxDelay           = 50 * time.Millisecond
	WildTenantLifetime = 500 * time.Millisecond
	BoardWidth         = 15
	BoardHeight        = 15
	MaxStepsBuffer     = MaxSteps + 100
	DEBUG              = false
	TrapsCount         = 10
)

type Position struct {
	X, Y int
}

type Player struct {
	ID       int
	Symbol   rune
	Position Position
	Wild     bool
}

type Trace struct {
	Timestamp time.Duration
	ID        int
	Position  Position
	Symbol    rune
}

type AtomicCounter struct {
	count int
	mu    sync.Mutex
}

func (c *AtomicCounter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func (c *AtomicCounter) Decrement() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count--
}

func (c *AtomicCounter) GetCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

type Trap struct {
	Position Position
	ID       int
}

type Cell struct {
	commandChan chan func()
	locked      bool
	occupied    bool
	trapped     bool
	occupant    int
	trap        *Trap
	traces      []Trace
}

func NewCell() *Cell {
	c := &Cell{
		commandChan: make(chan func()),
	}
	go c.run()
	return c
}

func (c *Cell) run() {
	for cmd := range c.commandChan {
		if DEBUG {
			fmt.Printf("Cell starts\n")
		}
		cmd()
		if DEBUG {
			fmt.Printf("Cell ends\n")
		}
	}
}

func (c *Cell) Lock() bool {
	if DEBUG {
		fmt.Printf("Locking\n")
	}
	result := make(chan bool, 1)
	c.commandChan <- func() {
		if !c.locked {
			c.locked = true
			result <- true
		} else {
			result <- false
		}
	}
	return <-result
}

func (c *Cell) AddTrap(ID int, X int, Y int) {
	c.commandChan <- func() {
		c.trapped = true
		c.trap = &Trap{
			Position: Position{X: X, Y: Y},
			ID:       ID,
		}
	}
}

func (c *Cell) CheckTrap() bool {
	result := make(chan bool, 1)
	c.commandChan <- func() {
		result <- c.trapped
	}
	return <-result
}

func (c *Cell) Unlock() {
	if DEBUG {
		fmt.Printf("Unlocking\n")
	}
	c.commandChan <- func() {
		c.locked = false
	}
}

func (c *Cell) IsLocked() bool {
	if DEBUG {
		fmt.Printf("IsLocked\n")
	}
	result := make(chan bool, 1)
	c.commandChan <- func() {
		result <- c.locked
	}
	return <-result
}

func (c *Cell) Occupy(mover int) {
	if DEBUG {
		fmt.Printf("Occupy\n")
	}
	c.commandChan <- func() {
		c.occupant = mover
		c.occupied = true
	}
}

func (c *Cell) IsOccupied() bool {
	if DEBUG {
		fmt.Printf("IsOccupied\n")
	}
	result := make(chan bool, 1)
	c.commandChan <- func() {
		result <- c.occupied
	}
	return <-result
}

func (c *Cell) GetOccupant() int {
	if DEBUG {
		fmt.Printf("GetOccupant\n")
	}
	result := make(chan int, 1)
	c.commandChan <- func() {
		result <- c.occupant
	}
	return <-result
}

func (c *Cell) Clear() {
	if DEBUG {
		fmt.Printf("Clear\n")
	}
	c.commandChan <- func() {
		c.occupied = false
	}
}

func (c *Cell) storeTrace() {
	if DEBUG {
		fmt.Printf("storeTrace\n")
	}
	c.commandChan <- func() {
		player := players[c.occupant]
		if c.occupied {
			c.traces = append(c.traces, Trace{
				Timestamp: time.Since(startTime),
				ID:        player.ID,
				Position:  player.Position,
				Symbol:    player.Symbol,
			})
		} else if c.trapped {
			c.traces = append(c.traces, Trace{
				Timestamp: time.Since(startTime),
				ID:        c.trap.ID,
				Position:  c.trap.Position,
				Symbol:    '#',
			})
		}
	}
}

func (c *Cell) ExportTraces() []Trace {
	if DEBUG {
		fmt.Printf("ExportTraces\n")
	}
	result := make(chan []Trace, 1)
	c.commandChan <- func() {
		traces := make([]Trace, len(c.traces))
		copy(traces, c.traces)
		result <- traces
	}
	return <-result
}

func (c *Cell) MoveWildTenant(cellX, cellY int, board [][]*Cell) bool {
	if DEBUG {
		fmt.Printf("MoveWild\n")
	}
	result := make(chan bool, 1)
	select {
	case c.commandChan <- func() {
		if c.occupied && players[c.occupant].Wild {
			// Get safe reference to player
			player := players[c.occupant]
			dirs := []Position{
				{1, 0}, {-1, 0}, {0, 1}, {0, -1}, // Right, Left, Down, Up
			}
			rand.Shuffle(len(dirs), func(i, j int) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			})

			for _, dir := range dirs {
				newX := (cellX + dir.X + BoardWidth) % BoardWidth
				newY := (cellY + dir.Y + BoardHeight) % BoardHeight

				if board[newX][newY].Lock() {
					if !board[newX][newY].IsOccupied() {
						board[newX][newY].Occupy(c.occupant)
						player.Position = Position{newX, newY}
						c.occupied = false
						if board[newX][newY].CheckTrap() {
							// Trap activated - freeze traveler
							players[c.occupant].Symbol = '*'
							board[newX][newY].storeTrace()
							time.Sleep(MinDelay)

							// Traveler dies
							players[c.occupant].Position.X = -1
							players[c.occupant].Position.Y = -1
							board[newX][newY].Clear()
						}
						board[newX][newY].storeTrace()
						board[newX][newY].Unlock()
						result <- true
						return
					}
					board[newX][newY].Unlock()
				}
			}

			result <- false
		} else {
			result <- true
		}
	}:
		return <-result
	case <-time.After(MaxDelay * 2):
		return false
	}
}

var (
	players           []*Player
	activeTravelers   AtomicCounter
	activeWildTenants AtomicCounter
	startTime         time.Time
	printerChan       chan []Trace
	wg                sync.WaitGroup
)

func travelerTask(id int, seed int64) {
	defer wg.Done()
	<-startSignal

	r := rand.New(rand.NewSource(seed))
	traveler := id
	nrOfSteps := MinSteps + r.Intn(MaxSteps-MinSteps+1)
	var traces []Trace

	storeTrace := func() {
		traces = append(traces, Trace{
			Timestamp: time.Since(startTime),
			ID:        players[traveler].ID,
			Position:  players[traveler].Position,
			Symbol:    players[traveler].Symbol,
		})
	}

	makeStep := func() {
		n := r.Intn(4)
		var newX, newY int
		currentX := players[traveler].Position.X
		currentY := players[traveler].Position.Y

		switch n {
		case 0:
			newX = currentX
			newY = (currentY + BoardHeight - 1) % BoardHeight
		case 1:
			newX = currentX
			newY = (currentY + 1) % BoardHeight
		case 2:
			newX = (currentX + BoardWidth - 1) % BoardWidth
			newY = currentY
		case 3:
			newX = (currentX + 1) % BoardWidth
			newY = currentY
		}

		if board[newX][newY].Lock() {
			if DEBUG {
				fmt.Printf("%d locked %d %d\n", id, newX, newY)
			}
			if board[newX][newY].MoveWildTenant(newX, newY, board) {
				if DEBUG {
					fmt.Printf("%d emptied %d %d\n", id, newX, newY)
				}
				board[newX][newY].Occupy(traveler)
				players[traveler].Position.X = newX
				players[traveler].Position.Y = newY
				// Check for trap activation
				if board[newX][newY].CheckTrap() {
					// Trap activated - freeze traveler
					players[traveler].Symbol = unicode.ToLower(players[traveler].Symbol)
					board[newX][newY].storeTrace()
					time.Sleep(MinDelay)

					// Traveler dies
					players[traveler].Position.X = -1
					players[traveler].Position.Y = -1
					board[newX][newY].Clear()
					board[newX][newY].storeTrace()
				}
				storeTrace()
				board[currentX][currentY].Unlock()
			} else {
				if unicode.IsUpper(players[traveler].Symbol) {
					players[traveler].Symbol = unicode.ToLower(players[traveler].Symbol)
				}
				storeTrace()
				board[newX][newY].Unlock()
			}
		} else {
			if unicode.IsUpper(players[traveler].Symbol) {
				players[traveler].Symbol = unicode.ToLower(players[traveler].Symbol)
			}
			storeTrace()
		}
	}

	for step := 0; step < nrOfSteps; step++ {
		delay := MinDelay + time.Duration(float64(MaxDelay-MinDelay)*r.Float64())
		time.Sleep(delay)
		makeStep()

		if !unicode.IsUpper(players[traveler].Symbol) {
			break
		}
	}

	if unicode.IsUpper(players[traveler].Symbol) {
		players[traveler].Symbol = unicode.ToLower(players[traveler].Symbol)
		storeTrace()
	}

	activeTravelers.Decrement()
	printerChan <- traces
}

func wildTenantTask(id int, seed int64) {
	defer wg.Done()
	<-startSignal

	r := rand.New(rand.NewSource(seed))
	wildTenant := id
	var traces []Trace
	alive := false
	var birthTime time.Time

	storeTrace := func() {
		traces = append(traces, Trace{
			Timestamp: time.Since(startTime),
			ID:        players[wildTenant].ID,
			Position:  players[wildTenant].Position,
			Symbol:    players[wildTenant].Symbol,
		})
	}

	safeAppear := func() {
		for attempt := 0; attempt < 10; attempt++ {
			x := r.Intn(BoardWidth)
			y := r.Intn(BoardHeight)

			if board[x][y].Lock() {
				if !board[x][y].IsOccupied() {
					if !board[x][y].CheckTrap() {
						players[wildTenant].Position.X = x
						players[wildTenant].Position.Y = y
						players[wildTenant].Symbol = rune('0' + byte(players[wildTenant].ID%10))
						board[x][y].Occupy(wildTenant)
						alive = true
						birthTime = time.Now()
						storeTrace()
						board[x][y].Unlock()
						return
					}
				}
				board[x][y].Unlock()
			}
			time.Sleep(WildTenantLifetime / 10)
		}
	}

	safeDisappear := func() {
		x := players[wildTenant].Position.X
		y := players[wildTenant].Position.Y

		if x == -1 {
			alive = false
		} else if board[x][y].Lock() {
			if board[x][y].GetOccupant() == wildTenant {
				players[wildTenant].Position.X = -1
				players[wildTenant].Position.Y = -1
				board[x][y].Clear()
				storeTrace()
				alive = false
			}
			board[x][y].Unlock()
		}
	}

	for {
		if !alive {
			if DEBUG {
				fmt.Printf("APPEARING\n")
			}
			safeAppear()
		} else {
			if time.Since(birthTime) > WildTenantLifetime {
				safeDisappear()
			}
		}

		if activeTravelers.GetCount() < 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	activeWildTenants.Decrement()
	printerChan <- traces
}

var (
	board       [][]*Cell
	startSignal chan struct{}
)

func main() {
	startTime = time.Now()
	printerChan = make(chan []Trace, 1000)
	startSignal = make(chan struct{})
	wg = sync.WaitGroup{}

	players = make([]*Player, NrOfTravelers+NrOfWildTenants)
	for i := 0; i < NrOfTravelers; i++ {
		players[i] = &Player{
			ID:       i,
			Symbol:   rune('A' + i),
			Wild:     false,
			Position: Position{X: -1, Y: -1}, // Temporary invalid position
		}
	}

	// Create wild tenants (0-9)
	for i := 0; i < NrOfWildTenants; i++ {
		players[NrOfTravelers+i] = &Player{
			ID:     NrOfTravelers + i,
			Symbol: rune('0' + i%10),
			Wild:   true,
		}
	}

	// Initialize board
	board = make([][]*Cell, BoardWidth)
	for i := range board {
		board[i] = make([]*Cell, BoardHeight)
		for j := range board[i] {
			board[i][j] = NewCell()
		}
	}

	// Placing traps in different positions
	var allPositions []Position
	for x := 0; x < BoardWidth; x++ {
		for y := 0; y < BoardHeight; y++ {
			allPositions = append(allPositions, Position{X: x, Y: y})
		}
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(allPositions), func(i, j int) {
		allPositions[i], allPositions[j] = allPositions[j], allPositions[i]
	})
	for i := 0; i < TrapsCount && i < len(allPositions); i++ {
		pos := allPositions[i]
		board[pos.X][pos.Y].AddTrap(NrOfTravelers+NrOfWildTenants+i, pos.X, pos.Y)
		board[pos.X][pos.Y].storeTrace()
		if DEBUG {
			fmt.Printf("Placed trap at (%d,%d)\n", pos.X, pos.Y)
		}
	}

	// Print parameters
	fmt.Printf("-1 %d %d %d\n", NrOfTravelers+NrOfWildTenants+TrapsCount, BoardWidth, BoardHeight)

	// Start printer as a separate goroutine
	printerDone := make(chan struct{})
	go func() {
		for traces := range printerChan {
			for _, trace := range traces {
				fmt.Printf("%.9f %d %d %d %c\n",
					trace.Timestamp.Seconds(),
					trace.ID,
					trace.Position.X,
					trace.Position.Y,
					trace.Symbol)
			}
		}
		close(printerDone)
	}()

	// Place travelers randomly
	for i := 0; i < NrOfTravelers; i++ {
		for {
			x, y := rand.Intn(BoardWidth), rand.Intn(BoardHeight)
			if board[x][y].Lock() {
				if !board[x][y].IsOccupied() {
					board[x][y].Occupy(i)
					players[i].Position = Position{X: x, Y: y}
					board[x][y].storeTrace() // Record initial placement
					board[x][y].Unlock()
					break
				}
				board[x][y].Unlock()
			}
			time.Sleep(1 * time.Millisecond) // Avoid tight loop
		}
	}

	// Initialize traveler tasks
	wg.Add(NrOfTravelers)
	for i := 0; i < NrOfTravelers; i++ {
		activeTravelers.Increment()
		seed := time.Now().UnixNano() + int64(i)
		go travelerTask(i, seed)
	}

	// Initialize wild tenant tasks
	wg.Add(NrOfWildTenants)
	for i := 0; i < NrOfWildTenants; i++ {
		activeWildTenants.Increment()
		seed := time.Now().UnixNano() + int64(i+NrOfTravelers)
		go wildTenantTask(NrOfTravelers+i, seed)
	}

	// Start all tasks
	close(startSignal)

	// Wait for completion
	wg.Wait()

	// Collect all cell traces
	cellTracesWG := sync.WaitGroup{}
	for x := 0; x < BoardWidth; x++ {
		for y := 0; y < BoardHeight; y++ {
			cellTracesWG.Add(1)
			go func(x, y int) {
				defer cellTracesWG.Done()
				traces := board[x][y].ExportTraces()
				if len(traces) > 0 {
					printerChan <- traces
				}
			}(x, y)
		}
	}

	// Wait for all cell traces to be sent
	cellTracesWG.Wait()

	// Cleanup
	close(printerChan) // Signal printer to exit
	<-printerDone      // Wait for printer to finish
}
