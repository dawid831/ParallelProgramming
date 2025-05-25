package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
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
	value int32
}

func (mt *MaxTicket) Lock() {
	mt.mu.Lock()
}

func (mt *MaxTicket) Unlock() {
	mt.mu.Unlock()
}

func (mt *MaxTicket) Read() int32 {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	return mt.value
}

func (mt *MaxTicket) TryValue(newValue int32) {
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
	MyMaxTicket  int32
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
	choosing []int32
	number   []int32

	biggestTicket MaxTicket
	wg            sync.WaitGroup
	printerWG     sync.WaitGroup
	boardWidth    = NrOfProcesses
	boardHeight   = int(ExitProtocol) + 1
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Shared arrays
	choosing = make([]int32, NrOfProcesses)
	number = make([]int32, NrOfProcesses)

	// Create processes
	processes := make([]*Process, NrOfProcesses)
	for i := 0; i < NrOfProcesses; i++ {
		p := &Process{
			ID:        i,
			Symbol:    rune('A' + i),
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
	id := p.ID

	for step := 0; step < p.Steps/4; step++ {
		// Local Section
		p.recordState(LocalSection)
		p.randomDelay()

		// Entry Protocol (Bakery algorithm)
		p.recordState(EntryProtocol)

		atomic.StoreInt32(&choosing[id], 1)
		max := findMax()
		atomic.StoreInt32(&number[id], int32(max+1))
		atomic.StoreInt32(&choosing[id], 0)

		if atomic.LoadInt32(&number[id]) > p.MyMaxTicket {
			p.MyMaxTicket = atomic.LoadInt32(&number[id])
		}

		for j := 0; j < NrOfProcesses; j++ {
			if j == p.ID {
				continue
			}

			for atomic.LoadInt32(&choosing[j]) == 1 {
				time.Sleep(MinDelayMs / 10 * time.Millisecond)
			}

			for atomic.LoadInt32(&number[j]) != 0 &&
				(atomic.LoadInt32(&number[id]) > atomic.LoadInt32(&number[j]) ||
					(atomic.LoadInt32(&number[id]) == atomic.LoadInt32(&number[j]) && id > j)) {
				time.Sleep(MinDelayMs / 10 * time.Millisecond)
			}
		}

		// Critical Section
		p.recordState(CriticalSection)
		p.randomDelay()

		// Exit Protocol
		p.recordState(ExitProtocol)
		atomic.StoreInt32(&number[id], 0)
	}

	// Update global max ticket
	biggestTicket.TryValue(p.MyMaxTicket)
}

func findMax() int32 {
	max := int32(0)
	for i := 0; i < NrOfProcesses; i++ {
		if n := atomic.LoadInt32(&number[i]); n > max {
			max = n
		}
	}
	return max
}

func (p *Process) randomDelay() {
	delayMs := MinDelayMs + p.random.Intn(MaxDelayMs-MinDelayMs+1)
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
}

func (p *Process) recordState(state ProcessState) {
	stamp := time.Since(p.startTime)
	p.stateChanges = append(p.stateChanges, Trace{
		Timestamp: stamp,
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
