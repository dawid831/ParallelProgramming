package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Constants
const (
	NrOfProcesses = 2
	MinSteps      = 150
	MaxSteps      = 300
	MinDelay      = 10 * time.Millisecond
	MaxDelay      = 50 * time.Millisecond
)

// ProcessState represents the states of a process
type ProcessState int

const (
	LocalSection ProcessState = iota
	EntryProtocol
	CriticalSection
	ExitProtocol
)

// Board dimensions
const (
	BoardWidth  = NrOfProcesses
	BoardHeight = int(ExitProtocol) + 1
)

// Global variables
var (
	startTime time.Time
	c1        int32 = 1
	c2        int32 = 1
	turn      int32 = 1
)

// PositionType represents positions on the board
type PositionType struct {
	X int // 0..BoardWidth-1
	Y int // 0..BoardHeight-1
}

// TraceType represents traces of processes
type TraceType struct {
	TimeStamp time.Duration
	ID        int
	Position  PositionType
	Symbol    rune
}

// TracesSequenceType represents a sequence of traces
type TracesSequenceType struct {
	Last       int
	TraceArray []TraceType
}

// ProcessType represents a process
type ProcessType struct {
	ID       int
	Symbol   rune
	Position PositionType
}

// ProcessTask represents a process task
type ProcessTask struct {
	process     ProcessType
	rand        *rand.Rand
	nrOfSteps   int
	traces      TracesSequenceType
	tracesMutex sync.Mutex
	initDone    chan struct{}
	startDone   chan struct{}
	reportDone  chan struct{}
}

// NewProcessTask creates a new ProcessTask
func NewProcessTask(id int, seed int64, symbol rune) *ProcessTask {
	return &ProcessTask{
		process: ProcessType{
			ID:     id,
			Symbol: symbol,
			Position: PositionType{
				X: id,
				Y: int(LocalSection),
			},
		},
		rand:      rand.New(rand.NewSource(seed)),
		nrOfSteps: MinSteps + rand.Intn(MaxSteps-MinSteps+1),
		traces:    TracesSequenceType{Last: -1, TraceArray: make([]TraceType, MaxSteps+1)},

		initDone:   make(chan struct{}),
		startDone:  make(chan struct{}),
		reportDone: make(chan struct{}),
	}
}

// Init initializes the process task
func (pt *ProcessTask) Init() {
	// Store initial position
	pt.storeTrace()
	close(pt.initDone)
}

// Start starts the process task
func (pt *ProcessTask) Start() {
	<-pt.initDone
	close(pt.startDone)
}

// Run runs the process task
func (pt *ProcessTask) Run() {
	<-pt.startDone

	i := pt.process.ID

	for step := 0; step < pt.nrOfSteps/4; step++ { // Adjusted for testing
		// LOCAL_SECTION - start
		delay := MinDelay + time.Duration(float64(MaxDelay-MinDelay)*pt.rand.Float64())
		time.Sleep(delay)
		// LOCAL_SECTION - end

		pt.changeState(EntryProtocol) // starting ENTRY_PROTOCOL

		if i == 0 {
			atomic.StoreInt32(&c1, 0)
			for {

				if atomic.LoadInt32(&c2) != 0 {
					break
				}
				if atomic.LoadInt32(&turn) == 2 {
					atomic.StoreInt32(&c1, 1)
					for {
						if atomic.LoadInt32(&turn) != 2 {
							break
						}
					}
					atomic.StoreInt32(&c1, 0)
				}
			}
		} else {
			atomic.StoreInt32(&c2, 0)
			for {
				if atomic.LoadInt32(&c1) != 0 {
					break
				}
				if atomic.LoadInt32(&turn) == 1 {
					atomic.StoreInt32(&c2, 1)
					for {
						if atomic.LoadInt32(&turn) != 1 {
							break
						}
					}
					atomic.StoreInt32(&c2, 0)
				}
			}
		}

		pt.changeState(CriticalSection) // starting CRITICAL_SECTION

		// CRITICAL_SECTION - start
		delay = MinDelay + time.Duration(float64(MaxDelay-MinDelay)*pt.rand.Float64())
		time.Sleep(delay)
		// CRITICAL_SECTION - end

		pt.changeState(ExitProtocol) // starting EXIT_PROTOCOL
		if i == 0 {
			atomic.StoreInt32(&c1, 1)
			atomic.StoreInt32(&turn, 2)
		} else {
			atomic.StoreInt32(&c2, 1)
			atomic.StoreInt32(&turn, 1)
		}
		pt.changeState(LocalSection) // starting LOCAL_SECTION
	}

	pt.reportDone <- struct{}{}
}

func (pt *ProcessTask) storeTrace() {
	pt.tracesMutex.Lock()
	defer pt.tracesMutex.Unlock()

	pt.traces.Last++
	pt.traces.TraceArray[pt.traces.Last] = TraceType{
		TimeStamp: time.Since(startTime),
		ID:        pt.process.ID,
		Position:  pt.process.Position,
		Symbol:    pt.process.Symbol,
	}
}

func (pt *ProcessTask) changeState(state ProcessState) {
	pt.process.Position.Y = int(state)
	pt.storeTrace()
}

// Printer collects and prints reports of traces
func printer(reportChan <-chan TracesSequenceType, wg *sync.WaitGroup) {
	defer wg.Done()

	// Collect and print the traces
	for i := 0; i < NrOfProcesses; i++ {
		traces := <-reportChan
		printTraces(traces)
	}

	// Print the line with the parameters needed for display script
	fmt.Printf("-1 %d %d %d ", NrOfProcesses, BoardWidth, BoardHeight)
	for state := LocalSection; state <= ExitProtocol; state++ {
		fmt.Printf("%s;", state)
	}
	fmt.Println("EXTRA_LABEL;") // Place labels with extra info here
}

func printTrace(trace TraceType) {
	fmt.Printf("%.9f %d %d %d %c\n",
		trace.TimeStamp.Seconds(),
		trace.ID,
		trace.Position.X,
		trace.Position.Y,
		trace.Symbol)
}

func printTraces(traces TracesSequenceType) {
	for i := 0; i <= traces.Last; i++ {
		printTrace(traces.TraceArray[i])
	}
}

func main() {
	startTime = time.Now()

	// Create process tasks
	processTasks := make([]*ProcessTask, NrOfProcesses)
	symbol := 'A'

	for i := 0; i < NrOfProcesses; i++ {
		seed := time.Now().UnixNano() + int64(i)*1000
		processTasks[i] = NewProcessTask(i, seed, symbol)
		symbol++
	}

	// Initialize process tasks
	for _, pt := range processTasks {
		pt.Init()
	}

	// Start process tasks
	for _, pt := range processTasks {
		pt.Start()
	}

	// Create a channel for reports and a wait group for the printer
	reportChan := make(chan TracesSequenceType, NrOfProcesses)
	var wg sync.WaitGroup
	wg.Add(1)
	go printer(reportChan, &wg)

	// Run process tasks and collect reports
	for _, pt := range processTasks {
		go pt.Run()
	}

	// Wait for all process tasks to finish and send reports
	for _, pt := range processTasks {
		<-pt.reportDone
		pt.tracesMutex.Lock()
		reportChan <- pt.traces
		pt.tracesMutex.Unlock()
	}

	close(reportChan)
	wg.Wait()
}

// String representation of ProcessState
func (ps ProcessState) String() string {
	switch ps {

	case LocalSection:
		return "LOCAL_SECTION"
	case EntryProtocol:
		return "ENTRY_PROTOCOL"
	case CriticalSection:
		return "CRITICAL_SECTION"
	case ExitProtocol:
		return "EXIT_PROTOCOL"
	default:
		return "UNKNOWN_STATE"
	}
}
