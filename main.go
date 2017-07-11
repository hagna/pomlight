// This file is subject to a 1-clause BSD license.
// Its contents can be found in the enclosed LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jteeuwen/evdev"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

var pauseCommand = flag.String("pause", "/home/pi/pause", "The name of the pause script to call")
var beginCommand = flag.String("begin", "/home/pi/begin", "The name of the timer script")
var endCommand = flag.String("end", "/home/pi/end", "The end of the pomodoro time")

type fn func(ev *evdev.Event) fn

type Fsm struct {
	ctx           context.Context
	lastev        *evdev.Event
	pause         bool
	State         fn
	elapsedString string
	elapsed       float64
	cancel        context.CancelFunc
	StartTime     time.Time
	done          chan bool
	Input         chan evdev.Event
}

func NewFsm() *Fsm {
	f := new(Fsm)
	f.State = f.stInit
	f.done = make(chan bool)
	return f
}

func (fsm *Fsm) stInit(ev *evdev.Event) fn {
	log.Println("Init")
	if ev == nil {
		return fsm.stInit
	}
	fmt.Println(ev)
	if ev.Value == 1 {
		fsm.lastev = ev
		fsm.elapsedString = "0"
		if fsm.cancel != nil {
			fsm.cancel()
		}
		return fsm.stStart
	}
	return fsm.stInit
}

func (fsm *Fsm) stStart(ev *evdev.Event) fn {
	log.Println("Start")
	log.Printf("%+v", ev)
	if ev.Value == 0 && ev.Code == fsm.lastev.Code {
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, *beginCommand, fsm.elapsedString)
		log.Printf("%s %s", *beginCommand, fsm.elapsedString)
		cmd.Stdin = nil
		go func() {
			if err := cmd.Run(); err != nil {
				log.Println(err)
			} else {
				fsm.done <- true
			}
		}()
		fsm.StartTime = time.Now()
		fsm.ctx = ctx
		fsm.cancel = cancel
		return fsm.stTimer(nil)
	}
	return fsm.stStart
}

func (fsm *Fsm) stEnd(ev *evdev.Event) fn {
	log.Println("End")
	ctx, cancel := context.WithCancel(context.Background())
	if err := exec.CommandContext(ctx, *endCommand, fsm.elapsedString).Start(); err != nil {
		log.Println(err)
		return fsm.stInit
	}
	fsm.cancel = cancel
	return fsm.stInit(ev)
}

func (fsm *Fsm) stTimer(ign *evdev.Event) fn {
	select {
	case ev := <-fsm.Input:
		if ev.Value == 1 {
			fsm.lastev = &ev

			return fsm.stPause
		}
	case <-fsm.done:
		log.Println("context is done")
		return fsm.stEnd(nil)
	case <-time.After(5 * time.Second):
		fmt.Printf("tick ")
	}
	return fsm.stTimer(nil)
}

func (fsm *Fsm) stPause(ev *evdev.Event) fn {
	fsm.pause = !fsm.pause
	if fsm.pause {
		log.Println("Pause")
		fsm.cancel()
		ctx, cancel := context.WithCancel(context.Background())
		if err := exec.CommandContext(ctx, *pauseCommand).Start(); err != nil {
			log.Println(err)
			return fsm.stInit

		}

		elapsed := time.Since(fsm.StartTime)
		fsm.elapsed += elapsed.Seconds()
		fsm.elapsedString = strconv.FormatFloat(elapsed.Seconds(), 'f', -1, 64)
		fsm.cancel = cancel
		fsm.ctx = ctx
	} else {
		// unpause
		log.Println("Unpause")
		fsm.cancel()
		return fsm.stStart(ev)
	}
	return fsm.stTimer(nil)
}

func main() {
	node := parseArgs()

	// Create and open our device.
	dev, err := evdev.Open(node)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	// Make sure it is closed once we are done.
	defer dev.Close()

	// Ensure this device supports key/button events.
	if !dev.Test(dev.EventTypes(), evdev.EvKeys) {
		fmt.Fprintf(os.Stderr, "Device %q does not support key/button events.\n", node)
		return
	}

	// Fetch the current key/button state and display it.
	ks := dev.KeyState()
	listState(ks)

	// Alternatively, we can read individual key/button press/release events
	// from the device.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	fsm := NewFsm()
	fsm.Input = dev.Inbox
	for {
		select {
		case <-signals:
			return

		case evt := <-dev.Inbox:

			if evt.Type != evdev.EvKeys {
				continue // Not a key event.
			}
			state := fsm.State(&evt)
			fsm.State = state

		}
	}
}

// listState prints the global key/button state, as defined
// in the given bitset.
func listState(set evdev.Bitset) {
	for n := 0; n < set.Len(); n++ {
		// The key is considered pressed if the bitset
		// has its corresponding bit set.
		if !set.Test(n) {
			continue
		}

		fmt.Printf("  Key 0x%02x ", n)

		switch n {
		case evdev.KeyReserved:
			fmt.Printf("Reserved")
		case evdev.KeyEscape:
			fmt.Printf("Escape")
		case evdev.BtnStylus2:
			fmt.Printf("2nd stylus button")

			// more keys/buttons..
		}

		fmt.Println()
	}
}

func parseArgs() string {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s <node>\n", os.Args[0])
		os.Exit(1)
	}

	return flag.Args()[0]
}
