# pomlight overview

Pomodoro timers can help you focus when focusing is the last thing you want to do, or almost the last thing you want to do.  Simply set the pomodoro timer and work till it elapses.  Typically the timer is for 25 minutes.

## Architecture

Use the keyboard or mouse on the raspberry pi to trigger the timer to start or pause.  Yes, pause for interruptions.  Display something unique on the unicorn phat for the different states of running, paused, and ended.

## Implementation

Use evdev (/dev/input/event*) on linux for keyboard input.
pomlight.go uses a go wrapper for evdev. Here is a python one https://python-evdev.readthedocs.io/en/latest/

In this repo main.go implements the state machine.  Run it like this:
./pomlight /dev/input/event0

It will run the programs /home/pi/begin for the running state, /home/pi/pause for the pause state, and /home/pi/end for the end state.


