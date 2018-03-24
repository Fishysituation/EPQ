package main

import (
	"time"

	"github.com/go-rpio-master"
)

//DECLARE PINS

var (
	//rack button
	rack = rpio.Pin(10)

	//help button
	help = rpio.Pin(11)

	//height counter pins
	c1 = rpio.Pin(5)
	c2 = rpio.Pin(6)
	c3 = rpio.Pin(13)
	c4 = rpio.Pin(19)

	//motor
	motor = rpio.Pin(4)
)

//program entry point
func main() {

	//open / map gpio memory
	rpio.Open()
	//init inputs
	rack.Input()
	c1.Input()
	c2.Input()
	c3.Input()
	c4.Input()
	//init outputs
	motor.Output()

	//loop indefinitely
	for {
		//check every 500ms
		time.Sleep(time.Millisecond * 500)

		//if barbell is off rack
		if rack.Read() == rpio.Low {
			//execute main program
			run()
		}
	}
}

//main program loop
func run() {
	//arbitrary height variable
	height := 50
	struggling := false

	//run state for checks - also used for sync
	run := make(chan bool)
	//run state for checkHeight
	runHeight := make(chan bool)
	//reciever message channel with buffer 5 to avoid "deadlock"
	in := make(chan string, 5)

	//run all checks as goroutines
	go updateHeight(&height, runHeight)
	go checkTime(&height, run, in)
	go checkFall(&height, run, in)
	go checkHelp(&struggling, run, in)
	go checkRack(run, in)

	//loop indefinitely
	for {
		select {

		//if message has come from in channel, handle it
		case msg := <-in:

			//if barbell has fallen
			if msg == "fall" {
				//kill all checks
				run <- false
				temp := height

				//wait on barbell height to increase (initial lift)
				//prevents holding motor at stall torque when lifter lifting barbell
				for {
					if height != temp {
						break
					}
				}
				//reracK barbell
				reRack(&height)
				//kill goroutine updateHeight when finished
				runHeight <- false
				return
			}

			//if lifter wants help
			if msg == "stop" {
				//kill all checks
				run <- false

				//rerack barbell
				reRack(&height)
				//kill goroutine updateHeight when finished
				runHeight <- false
				return
			}

			//if reracked
			if msg == "rack" {
				run <- false
				return
			}

			//if lifter is struggling
			if msg == "stuggle" {
				//update variable given to checkHelp
				struggling = true
			}

		default:
			//send regular operation to all
			run <- true
		}
	}
}

func reRack(height *int) {
	//loop indefinitely
	for {
		//turn motor on
		motor.Write(rpio.High)

		//if just under the original height is reached
		if *height >= 48 {
			break
			//if barbell has been reracked
		} else if rack.Read() == rpio.High {
			break
		}
	}
	//turn motor off
	motor.Write(rpio.Low)
}

//update pointer to height variable
func updateHeight(height *int, in chan bool) {

	prev := 0
	//get initial
	for {
		if c1.Read() == rpio.High {
			prev = 1
			break
		} else if c2.Read() == rpio.High {
			prev = 2
			break
		} else if c3.Read() == rpio.High {
			prev = 3
			break
		} else if c4.Read() == rpio.High {
			prev = 4
			break
		}
	}

	for {
		select {
		//if message has come in from in channel
		case msg := <-in:
			// ensure only quit if correct message sent through channel
			if msg == false {
				return
			}

		default:
			//check buttons adjacent to prev
			//increment/decrement height counter accordingly
			if prev == 1 {
				if c4.Read() == rpio.High {
					*height--
				} else if c2.Read() == rpio.High {
					*height++
				}

			} else if prev == 2 {
				if c1.Read() == rpio.High {
					*height--
				} else if c3.Read() == rpio.High {
					*height++
				}

			} else if prev == 3 {
				if c2.Read() == rpio.High {
					*height--
				} else if c4.Read() == rpio.High {
					*height++
				}

			} else if prev == 4 {
				if c3.Read() == rpio.High {
					*height--
				} else if c1.Read() == rpio.High {
					*height++
				}
			}
		}
	}
}

//look at the rep times for signs of struggle
func checkTime(height *int, in chan bool, out chan string) {

	prev2 := *height
	prev := *height
	t := time.Now()
	var initial time.Duration
	count := 0

	for {
		//wait on control signal
		msg := <-in
		//if okay to continue
		if msg == true {
			//if the height has changed
			if *height != prev {
				//push heights along
				prev2 = prev
				prev = *height
				//if prev is greater than both adjacent values
				if prev2 < prev && *height < prev {
					count++
					//if first rep
					if count == 1 {
						//record rep time
						initial = time.Since(t)
					}
					t = time.Now()
				}
				//if time taken is too great notify user and system
				if time.Since(t) > 2*initial {
					out <- "struggle"
				}
			}

			//if channel in is given false, quit
		} else {
			return
		}
	}
}

//check if the barbell has fallen
func checkFall(height *int, in chan bool, out chan string) {

	var times [5]time.Time
	var heights [5]int

	for {
		//wait on control signal
		msg := <-in
		//if okay to continue
		if msg == true {
			//if height has changed
			if *height != heights[4] {
				//push previous along
				for i := 0; i < 5; i++ {
					times[i] = times[i+1]
					heights[i] = heights[i+1]
				}
				//enter new time and height
				times[4] = time.Now()
				heights[4] = *height

				//if time since 5 height changes ago was less than 200ms
				if time.Since(times[0]) < time.Millisecond*500 {
					//at least fall of 4 units
					if heights[4]-heights[0] >= 4 {
						out <- "fall"
						return
					}
				}
			}

			//if channel in is given false, quit
		} else {
			return
		}
	}
}

//look at the help button
func checkHelp(struggle *bool, in chan bool, out chan string) {

	t := time.Now()
	isPressed := false
	for {
		//wait on control signal
		msg := <-in
		//if okay to continue
		if msg == true {
			//if help button is pressed
			if help.Read() == rpio.High {
				//if lifter is deemed to be struggling
				if *struggle == true {
					//send stop signal to main
					out <- "stop"
					return

					//if lifter isn't struggling and button has been held
				} else if isPressed == true {
					//if button held for over 2 seconds
					if time.Since(t) > time.Second*2 {
						//send stop signal to main
						out <- "stop"
						return
					}

					//if button not pressed in previous iteration
				} else if isPressed == false {
					//set press flag true
					isPressed = true
					//take current time
					t = time.Now()
				}

				//else the button is (no longer) being pressed
			} else {
				//set press flag false
				isPressed = false
			}

			//else if not ok
		} else {
			return
		}

	}
}

//check if barbell has be reracked
func checkRack(in chan bool, out chan string) {

	for {
		//wait on control signal
		msg := <-in
		//if okay to continue
		if msg == true {
			//if barbell has been re-racked
			if rack.Read() == rpio.High {
				out <- "rack"
				return
			}

			//if channel in is given false, quit
		} else {
			return
		}
	}
}
