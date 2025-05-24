package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	NrOfProcesses = 15
	MinSteps      = 50
	MaxSteps      = 100
	MinDelayMs    = 10
	MaxDelayMs    = 50
)

type ProcessState int

const (
	LocalSection ProcessState = iota
	EntryProtocol
	CriticalSection
	ExitProtocol
)

func (s ProcessState) String() string {
	return [...]string{"LOCAL_SECTION", "ENTRY_PROTOCOL", "CRITICAL_SECTION", "EXIT_PROTOCOL"}[s]
}

type MaxTicket struct {
	mu    sync.Mutex
	value int
}

func (mt *MaxTicket) Lock() {
	mt.mu.Lock()
}

func (mt *MaxTicket) Unlock() {
	mt.mu.Unlock()
}

func (mt *MaxTicket) Read() int {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.value
}

func (mt *MaxTicket) TryValue(newValue int) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if newValue > mt.value {
		mt.value = newValue
	}
}

type Process struct {
	ID           int
	Symbol       rune
	State        ProcessState
	Steps        int
	MyMaxTicket  int
	choosing     []int
	number       []int
	random       *rand.Rand
	stateChanges []Trace
	startTime    time.Time
}

type Trace struct {
	Timestamp time.Duration
	ID        int
	State     ProcessState
	Symbol    rune
}

var (
	biggestTicket MaxTicket
	wg            sync.WaitGroup
	printerWG     sync.WaitGroup
	boardWidth    = NrOfProcesses
	boardHeight   = int(ExitProtocol) + 1
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Shared arrays
	choosing := make([]int, NrOfProcesses)
	number := make([]int, NrOfProcesses)

	// Create processes
	processes := make([]*Process, NrOfProcesses)
	for i := 0; i < NrOfProcesses; i++ {
		p := &Process{
			ID:        i,
			Symbol:    rune('A' + i),
			choosing:  choosing,
			number:    number,
			random:    rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))),
			startTime: time.Now(),
		}
		p.Steps = MinSteps + p.random.Intn(MaxSteps-MinSteps+1)
		processes[i] = p
	}

	// Start printer
	printerWG.Add(1)
	go printer(processes)

	// Start processes
	for _, p := range processes {
		wg.Add(1)
		go p.Run()
	}

	// Wait for all processes to finish
	wg.Wait()

	// Signal printer to finish
	printerWG.Wait()
}

func (p *Process) Run() {
	defer wg.Done()

	for step := 0; step < p.Steps/4; step++ {
		// Local Section
		p.recordState(LocalSection)
		p.randomDelay()

		// Entry Protocol (Bakery algorithm)
		p.recordState(EntryProtocol)

		p.choosing[p.ID] = 1
		p.number[p.ID] = 1 + p.findMax()
		p.choosing[p.ID] = 0

		if p.number[p.ID] > p.MyMaxTicket {
			p.MyMaxTicket = p.number[p.ID]
		}

		for j := 0; j < NrOfProcesses; j++ {
			if j == p.ID {
				continue
			}

			for p.choosing[j] == 1 {
				// busy wait
			}

			for p.number[j] != 0 &&
				(p.number[p.ID] > p.number[j] ||
					(p.number[p.ID] == p.number[j] && p.ID > j)) {
				// busy wait
			}
		}

		// Critical Section
		p.recordState(CriticalSection)
		p.randomDelay()

		// Exit Protocol
		p.recordState(ExitProtocol)
		p.number[p.ID] = 0
	}

	// Update global max ticket
	biggestTicket.TryValue(p.MyMaxTicket)
}

func (p *Process) findMax() int {
	max := 0
	for _, num := range p.number {
		if num > max {
			max = num
		}
	}
	return max
}

func (p *Process) randomDelay() {
	delayMs := MinDelayMs + p.random.Intn(MaxDelayMs-MinDelayMs+1)
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
}

func (p *Process) recordState(state ProcessState) {
	p.stateChanges = append(p.stateChanges, Trace{
		Timestamp: time.Since(p.startTime),
		ID:        p.ID,
		State:     state,
		Symbol:    p.Symbol,
	})
	p.State = state
}

func printer(processes []*Process) {
	defer printerWG.Done()

	// Wait for all processes to finish
	wg.Wait()

	// Collect all state changes
	var allChanges []Trace
	for _, p := range processes {
		allChanges = append(allChanges, p.stateChanges...)
	}

	// Print all traces (similar to ADA version)
	for _, change := range allChanges {
		fmt.Printf("%.9f %d %d %d %c\n",
			change.Timestamp.Seconds(),
			change.ID,
			change.ID,         // X position (same as ID)
			int(change.State), // Y position
			change.Symbol)
	}

	// Print the parameters line (matches ADA output)
	fmt.Printf("-1 %d %d %d ", NrOfProcesses, NrOfProcesses, 4)
	for state := LocalSection; state <= ExitProtocol; state++ {
		fmt.Printf("%s;", state)
	}
	fmt.Printf("MAX_TICKET= %d;\n", biggestTicket.Read())
}
