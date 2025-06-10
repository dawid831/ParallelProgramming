package main

import (
	"fmt"
	"math/rand"
	"runtime"
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
	EntryProtocol_1
	EntryProtocol_2
	EntryProtocol_3
	EntryProtocol_4
	CriticalSection
	ExitProtocol
)

func (s ProcessState) String() string {
	return [...]string{"LOCAL_SECTION", "ENTRY_PROTOCOL_1", "ENTRY_PROTOCOL_2", "ENTRY_PROTOCOL_3", "ENTRY_PROTOCOL_4", "CRITICAL_SECTION", "EXIT_PROTOCOL"}[s]
}

type Process struct {
	ID           int
	Symbol       rune
	State        ProcessState
	Steps        int
	MyMaxTicket  int32
	random       *rand.Rand
	stateChanges []Trace
}

type Trace struct {
	Timestamp time.Duration
	ID        int
	State     ProcessState
	Symbol    rune
}

var (
	flags     []int32
	startTime time.Time

	wg          sync.WaitGroup
	printerWG   sync.WaitGroup
	boardWidth  = NrOfProcesses
	boardHeight = int(ExitProtocol) + 1
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// Shared arrays
	flags = make([]int32, NrOfProcesses)
	startTime = time.Now()

	// Create processes
	processes := make([]*Process, NrOfProcesses)
	for i := 0; i < NrOfProcesses; i++ {
		p := &Process{
			ID:     i,
			Symbol: rune('A' + i),
			random: rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))),
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

	p.recordState(LocalSection)
	for step := 0; step < p.Steps/7-1; step++ {
		// Local Section
		p.randomDelay()

		// Entry Protocol
		atomic.StoreInt32(&flags[id], 1)
		p.recordState(EntryProtocol_1)
		for {
			success := true
			for j := 0; j < NrOfProcesses; j++ {
				if flags[j] > 2 {
					success = false
					break
				}
			}
			if success {
				break
			}
			runtime.Gosched()
		}

		atomic.StoreInt32(&flags[id], 3)
		p.recordState(EntryProtocol_3)

		success := false
		for j := 0; j < NrOfProcesses; j++ {
			if flags[j] == 1 {
				success = true
				break
			}
		}

		if success {
			atomic.StoreInt32(&flags[p.ID], 2)
			p.recordState(EntryProtocol_2)

			for {
				success := false
				for j := 0; j < NrOfProcesses; j++ {
					if flags[j] == 4 {
						success = true
						break
					}
				}

				if success {
					break
				}
				runtime.Gosched()
			}
		}

		atomic.StoreInt32(&flags[p.ID], 4)
		p.recordState(EntryProtocol_4)

		for {
			success := true
			for j := 0; j < p.ID; j++ {
				if flags[j] > 1 {
					success = false
					break
				}
			}

			if success {
				break
			}
			runtime.Gosched()
		}

		p.recordState(CriticalSection)
		p.randomDelay()

		// Exit Protocol
		p.recordState(ExitProtocol)

		for {
			success := true
			for j := p.ID + 1; j < NrOfProcesses; j++ {
				if flags[j] == 2 || flags[j] == 3 {
					success = false
					break
				}
			}

			if success {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}

		atomic.StoreInt32(&flags[p.ID], 0)
		p.recordState(LocalSection)
	}
}

func (p *Process) randomDelay() {
	delayMs := MinDelayMs + p.random.Intn(MaxDelayMs-MinDelayMs+1)
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
}

func (p *Process) recordState(state ProcessState) {
	stamp := time.Since(startTime)
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

	for _, change := range allChanges {
		fmt.Printf("%.9f %d %d %d %c\n",
			change.Timestamp.Seconds(),
			change.ID,
			change.ID,         // X position (same as ID)
			int(change.State), // Y position
			change.Symbol)
	}

	fmt.Printf("-1 %d %d %d ", NrOfProcesses, NrOfProcesses, 7)
	for state := LocalSection; state <= ExitProtocol; state++ {
		fmt.Printf("%s;", state)
	}
	fmt.Println()
}
