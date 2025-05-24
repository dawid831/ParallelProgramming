package main

import "fmt"
import "time"
import "math/rand"

// stale wartosci
const (
	NrOfTravelers = 15
	MinSteps      = 10
	MaxSteps      = 100
	MinDelay      = 10 * time.Millisecond
	MaxDelay      = 50 * time.Millisecond
	BoardWidth    = 15
	BoardHeight   = 15
)

// typy
type Position struct {
	X, Y int
}

type Trace struct {
	TimeStamp time.Duration
	ID        int
	Position  Position
	Symbol    rune
}

type Traveler struct {
	ID       int
	Symbol   rune
	Position Position
	Traces   []Trace
}

// zmienne
var startTime = time.Now()

//funkcje ruchu
func (t *Traveler) moveUp() {
	t.Position.Y = (t.Position.Y + BoardHeight - 1) % BoardHeight
}

func (t *Traveler) moveDown() {
	t.Position.Y = (t.Position.Y + 1) % BoardHeight
}

func (t *Traveler) moveLeft() {
	t.Position.X = (t.Position.X + BoardWidth - 1) % BoardWidth
}

func (t *Traveler) moveRight() {
	t.Position.X = (t.Position.X + 1) % BoardWidth
}

func (t *Traveler) makeStep() {
	switch rand.Intn(4) {
	case 0:
		t.moveUp()
	case 1:
		t.moveDown()
	case 2:
		t.moveLeft()
	case 3:
		t.moveRight()
	}
}

// Przechowanie trace do wypisania
func (t *Traveler) storeTrace() {
	t.Traces = append(t.Traces, Trace{
		TimeStamp: time.Since(startTime),
		ID:        t.ID,
		Position:  t.Position,
		Symbol:    t.Symbol,
	})
}

// Funkcja do gorutyny symulujacej podroznika
func (t *Traveler) run(done chan<- bool) {
	nrOfSteps := MinSteps + rand.Intn(MaxSteps-MinSteps+1)
	for i := 0; i < nrOfSteps; i++ {
		time.Sleep(MinDelay + time.Duration(rand.Intn(int(MaxDelay-MinDelay))))
		t.makeStep()
		t.storeTrace()
	}
	printTraces(t.Traces)
	done <- true
}

// Wypisanie trace
func printTraces(traces []Trace) {
	for _, trace := range traces {
		fmt.Printf("%f %d %d %d %c\n", trace.TimeStamp.Seconds(), trace.ID, trace.Position.X, trace.Position.Y, trace.Symbol)
	}
}

func main() {
	fmt.Printf("-1 %d %d %d\n", NrOfTravelers, BoardWidth, BoardHeight)
	var travelers [NrOfTravelers]Traveler
	symbol := 'A'
	done := make(chan bool, NrOfTravelers)

	// Tworzenie podroznikow i ich danych
	for i := 0; i < NrOfTravelers; i++ {
		travelers[i] = Traveler{
			ID:       i,
			Symbol:   symbol,
			Position: Position{rand.Intn(BoardWidth), rand.Intn(BoardHeight)},
		}
		travelers[i].storeTrace()
		symbol++
	}

	// Start podroznikow w gorutynach
	for i := 0; i < NrOfTravelers; i++ {
		go travelers[i].run(done)
	}

	// Czeka na zakończenie wszystkich gorutyn
	for i := 0; i < NrOfTravelers; i++ {
		<-done
	}
}

