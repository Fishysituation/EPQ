package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-rpio-master"
)

//DECLARE PINS
var (
	//logger file path
	pathLog = "logs/log"

	//rack button
	rack = rpio.Pin(10)

	//help button
	help = rpio.Pin(11)

	//height counter pins
	c1 = rpio.Pin(5)
	c2 = rpio.Pin(6)
	c3 = rpio.Pin(13)
	c4 = rpio.Pin(19)
	//create sensor array for easier reference
	heightSensor = [4]rpio.Pin{c1, c2, c3, c4}

	//motor
	motor = rpio.Pin(12)

	//RGB pins (common anode)
	red   = rpio.Pin(16)
	green = rpio.Pin(20)
	blue  = rpio.Pin(21)
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
	red.Output()
	green.Output()
	blue.Output()

	//create new log file
	file := create(pathLog)

	//ensure red and blue off
	red.Write(rpio.High)
	blue.Write(rpio.High)

	//loop indefinitely
	for {
		//check every 500ms
		time.Sleep(time.Millisecond * 500)
		//keep green LED on
		green.Write(rpio.Low)

		//if barbell is off rack
		if rack.Read() == rpio.Low {
			//turn off green LED
			green.Write(rpio.High)
			//log
			log("\n SET START", file)
			//execute main program
			run(file)
			//log
			log("SET END \n", file)

			//turn red and blue off at end of set
			red.Write(rpio.High)
			blue.Write(rpio.High)
		}
	}
}

//
//FILE LOGGING/HANDLING
//makes new log file
func create(path string) *os.File {
	//if given path already exists
	file, err := os.Create(path + ".txt")
	//if collision
	if err != nil {
		//keep looping changing name until no collition
		count := 0
		for {
			//try to create file
			file1, err1 := os.Create(path + strconv.Itoa(count) + ".txt")
			//if exists
			if err1 != nil {
				//increment file no
				count++

				//else use file1
			} else {
				file = file1
			}
		}
	}
	return file
}

//writes message fileOut
func log(text string, file *os.File) {

	writer := bufio.NewWriter(file)
	defer file.Close()

	fmt.Fprintln(writer, text)
	writer.Flush()
}

//
//main operation loop
func run(file *os.File) {
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
		//keep blue (operation) led on
		blue.Write(rpio.Low)

		select {

		//if message has come from in channel, handle it
		case msg := <-in:

			//if barbell has fallen
			if msg == "fall" {
				//kill all checks
				run <- false
				temp := height
				log("barbell fallen", file)

				//turn off operation, turn on red
				blue.Write(rpio.High)
				red.Write(rpio.Low)

				//wait on barbell height to increase (initial lift)
				//or until user presses help button
				//prevents holding motor at stall
				for {
					if height != temp {
						log("barbell lifted", file)
						break
					}
					if help.Read() == rpio.High {
						log("help button pressed", file)
						break
					}
				}

				//reracK barbell
				//keep red LED flashing
				go flashRed(runHeight)
				reRack(&height)

				log("reracked safely", file)
				//kill goroutine updateHeight and flashRed when finished
				runHeight <- false
				return
			}

			//if lifter wants help
			if msg == "stop" {
				//kill all checks
				run <- false
				log("lifter asked for help", file)

				//turn off operation, turn on red
				blue.Write(rpio.High)
				red.Write(rpio.Low)

				//keep red LED flashing
				go flashRed(runHeight)
				//rerack barbell
				reRack(&height)

				log("reracked safely", file)
				//kill goroutine updateHeight and flashRed when finished
				runHeight <- false
				return
			}

			//if reracked
			if msg == "rack" {
				run <- false
				//turn off operation led
				blue.Write(rpio.High)
				log("barbell reracked", file)
				return
			}

			//if lifter is struggling
			if msg == "stuggle" {
				//update variable given to checkHelp
				struggling = true
				//strobe red led
				go askUser()
				log("lifter is struggling", file)
			}

			//if sending rep finished
			_, err := strconv.Atoi(msg)
			//if msg is numeric
			if err != nil {
				log("rep "+msg+" finished", file)
			}

		default:
			//send regular operation to all
			run <- true
		}
	}
}

//
//rerack the barbell
func reRack(height *int) {
	//loop indefinitely
	for {
		//turn motor on
		motor.Write(rpio.High)

		//if just under (around 3 cm) the original height is reached
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

//
//LED SIGNALS
//singal barbell is being reracked
func flashRed(in chan bool) {
	for {
		select {
		//if message sent over in channel
		case msg := <-in:
			//ensure correct exit signal is sent
			if msg == false {
				//turn off red LED
				red.Write(rpio.High)
				return
			}
		//else keep flashing LED
		default:
			red.Toggle()
			time.Sleep(time.Second)
		}
	}
}

//signal to user with RGB if they need help
func askUser() {
	//set blue led off
	blue.Write(rpio.High)
	//strobe red 5 times
	for i := 0; i < 5; i++ {
		red.Write(rpio.Low)
		time.Sleep(time.Millisecond * 500)
		red.Write(rpio.High)
		time.Sleep(time.Millisecond * 500)
	}
	//turm blue back on
	blue.Write(rpio.Low)
	return
}

//
//update pointer to height variable
func updateHeight(height *int, in chan bool) {

	prev := 0
	//get initial
	for {
		for i := 0; i < 4; i++ {
			if heightSensor[i].Read() == rpio.High {
				prev = i
			}
		}
		//break out of loop
		if prev != 0 {
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
			if heightSensor[(prev+3)%4].Read() == rpio.High {
				*height--
			}

			if heightSensor[(prev+5)%4].Read() == rpio.High {
				*height++
			}
		}
	}
}

//
//CHECK CONDITIONS
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
					//increment rep count
					count++
					//if first rep
					if count == 1 {
						//record rep time
						initial = time.Since(t)
					}
					t = time.Now()
					//send the rep number
					out <- strconv.Itoa(count)
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
