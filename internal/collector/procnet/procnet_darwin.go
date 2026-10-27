package procnet

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

func Supported() bool { return true }

type Collector struct {
	names map[int]string // PID -> app name, resolved once per process
}

func New() *Collector { return &Collector{names: map[int]string{}} }

func (c *Collector) Name() string { return "procnet" }

// Run streams `nettop` in delta mode: every Interval it prints a header line
// (starting with ",") followed by one line per process with the bytes it
// received and sent during that interval. The first block is different (totals
// since each process started) and is skipped.
//
// nettop runs under `script`, which gives it a pseudo-terminal. Writing to
// a pipe, the C library would buffer its output in large chunks, so samples
// would arrive in bursts, long after they were taken.
func (c *Collector) Run(ctx context.Context, out chan<- collector.Sample) error {
	cmd := exec.CommandContext(ctx, "script", "-q", "/dev/null",
		"nettop", "-P", "-d", "-x",
		// Only traffic on real network interfaces: without this, local
		// traffic (e.g. a browser talking to the hub over localhost) would
		// be blamed for using the internet.
		"-t", "external",
		"-L", "0", // keep sampling forever
		"-s", strconv.Itoa(int(Interval.Seconds())),
		"-J", "bytes_in,bytes_out")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start nettop: %w", err)
	}
	defer cmd.Wait() //nolint:errcheck // exits with ctx

	headers := 0
	var blockAt time.Time
	var block []entry
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := cleanLine(sc.Text())
		if !strings.HasPrefix(line, ",") {
			if e, ok := parseNettopLine(line); ok {
				block = append(block, e)
			}
			continue
		}
		// A header: the previous block is complete. It was printed when
		// its own header arrived, so that's its timestamp.
		if headers >= 2 {
			for _, s := range samples(summarize(block, Interval, c.appName), blockAt) {
				select {
				case out <- s:
				case <-ctx.Done():
					return nil
				}
			}
		}
		headers++
		blockAt, block = time.Now(), block[:0]
		if len(c.names) > 4096 { // PIDs get reused; start over now and then
			clear(c.names)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read nettop: %w", err)
	}
	return fmt.Errorf("nettop exited")
}

// appName resolves a process to its app through its executable path,
// falling back to nettop's truncated name.
func (c *Collector) appName(e entry) string {
	if name, ok := c.names[e.pid]; ok {
		return name
	}
	name := e.name
	if p, err := process.NewProcess(int32(e.pid)); err == nil {
		if exe, err := p.Exe(); err == nil && exe != "" {
			name = appName(exe)
		}
	}
	c.names[e.pid] = name
	return name
}
