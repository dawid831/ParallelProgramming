package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	NumReaders    = 10
	NumWriters    = 5
	NrOfProcesses = NumReaders + NumWriters
	MinSteps      = 20
	MaxSteps      = 40
	MinDelayMs    = 10
	MaxDelayMs    = 50
)

type ProcessState int

const (
	LocalSection ProcessState = iota
	Start
	ReadingRoom
	Stop
)

func (s ProcessState) String() string {
	return [...]string{"LOCAL_SECTION", "START", "READING_ROOM", "STOP"}[s]
}

type Process struct {
	ID           int
	Symbol       rune
	State        ProcessState
	Steps        int
	random       *rand.Rand
	stateChanges []Trace
}

type Trace struct {
	Timestamp time.Duration
	ID        int
	State     ProcessState
	Symbol    rune
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

func (p *Process) randomDelay() {
	delayMs := MinDelayMs + p.random.Intn(MaxDelayMs-MinDelayMs+1)
	time.Sleep(time.Duration(delayMs) * time.Millisecond)
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

	fmt.Printf("-1 %d %d %d ", NrOfProcesses, NrOfProcesses, 4)
	for state := LocalSection; state <= Stop; state++ {
		fmt.Printf("%s;", state)
	}
	fmt.Println()
}

// ########### MONITOR ###########
type request struct {
	response chan bool
}

type Monitor struct {
	enterChan chan struct{}
	condVars  map[string]*Condition
}

type Condition struct {
	queue   []*request
	monitor *Monitor
}

func NewMonitor() *Monitor {
	m := &Monitor{
		enterChan: make(chan struct{}, 1),
		condVars:  make(map[string]*Condition),
	}
	m.enterChan <- struct{}{}
	return m
}

func (m *Monitor) Enter() {
	<-m.enterChan
}

func (m *Monitor) Leave() {
	m.enterChan <- struct{}{}
}

func (m *Monitor) NewCondition(name string) *Condition {
	if _, exists := m.condVars[name]; !exists {
		m.condVars[name] = &Condition{
			queue:   make([]*request, 0),
			monitor: m,
		}
	}
	return m.condVars[name]
}

func (c *Condition) Wait() {
	req := request{response: make(chan bool)}
	c.queue = append(c.queue, &req)
	c.monitor.Leave()
	<-req.response
	c.monitor.Enter()
}

func (c *Condition) Signal() {
	c.monitor.Leave()
	if len(c.queue) > 0 {
		first := c.queue[0]
		c.queue = c.queue[1:]
		first.response <- true
	}
}

func (c *Condition) QueueLength() int {
	return len(c.queue)
}

// ########### RW ###########

type RWMonitor struct {
	monitor      *Monitor
	okToRead     *Condition
	okToWrite    *Condition
	readersCount int
	writing      bool
	waitWriters  int
}

func NewRWMonitor() *RWMonitor {
	m := NewMonitor()
	return &RWMonitor{
		monitor:      m,
		okToRead:     m.NewCondition("okToRead"),
		okToWrite:    m.NewCondition("okToWrite"),
		readersCount: 0,
		writing:      false,
		waitWriters:  0,
	}
}

func (rw *RWMonitor) StartRead() {
	rw.monitor.Enter()

	if rw.writing || rw.waitWriters > 0 {
		rw.okToRead.Wait()
	}

	rw.readersCount++
	rw.okToRead.Signal()
}

func (rw *RWMonitor) StopRead() {
	rw.monitor.Enter()

	rw.readersCount--
	if rw.readersCount == 0 {
		rw.okToWrite.Signal()
	} else {
		rw.monitor.Leave()
	}
}

func (rw *RWMonitor) StartWrite() {
	rw.monitor.Enter()
	rw.waitWriters++

	if rw.readersCount > 0 || rw.writing {
		rw.okToWrite.Wait()
	}

	rw.waitWriters--
	rw.writing = true
	rw.monitor.Leave()
}

func (rw *RWMonitor) StopWrite() {
	rw.monitor.Enter()

	rw.writing = false
	if rw.okToRead.QueueLength() > 0 {
		rw.okToRead.Signal()
	} else {
		rw.okToWrite.Signal()
	}
}

func (p *Process) reader(rw *RWMonitor) {
	defer wg.Done()

	p.recordState(LocalSection)
	for step := 0; step < p.Steps/4-1; step++ {
		// Local Section
		p.randomDelay()

		p.recordState(Start)
		rw.StartRead()

		p.recordState(ReadingRoom)
		p.randomDelay()

		p.recordState(Stop)
		rw.StopRead()

		p.recordState(LocalSection)
	}
}

func (p *Process) writer(rw *RWMonitor) {
	defer wg.Done()

	p.recordState(LocalSection)
	for step := 0; step < p.Steps/4-1; step++ {
		// Local Section
		p.randomDelay()

		p.recordState(Start)
		rw.StartWrite()

		p.recordState(ReadingRoom)
		p.randomDelay()

		p.recordState(Stop)
		rw.StopWrite()

		p.recordState(LocalSection)
	}
}

var (
	startTime time.Time

	wg          sync.WaitGroup
	printerWG   sync.WaitGroup
	boardWidth  = NrOfProcesses
	boardHeight = int(Stop) + 1
)

func main() {
	rand.Seed(time.Now().UnixNano())

	startTime = time.Now()

	processes := make([]*Process, NrOfProcesses)
	for i := 0; i < NumReaders; i++ {
		p := &Process{
			ID:     i,
			Symbol: rune('R'),
			random: rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))),
		}
		p.Steps = MinSteps + p.random.Intn(MaxSteps-MinSteps+1)
		processes[i] = p
	}

	for i := NumReaders; i < NrOfProcesses; i++ {
		p := &Process{
			ID:     i,
			Symbol: rune('W'),
			random: rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))),
		}
		p.Steps = MinSteps + p.random.Intn(MaxSteps-MinSteps+1)
		processes[i] = p
	}

	// Start printer
	printerWG.Add(1)
	go printer(processes)

	rw := NewRWMonitor()

	for i := 0; i < NumReaders; i++ {
		wg.Add(1)
		go processes[i].reader(rw)
	}

	for i := 0; i < NumWriters; i++ {
		wg.Add(1)
		go processes[NumReaders+i].writer(rw)
	}

	// Wait for all processes to finish
	wg.Wait()

	// Signal printer to finish
	printerWG.Wait()
}
