package main

import 
(
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"time"
)

//declare pins
//rack button
rack := rpio.pin()

//array for height counter


//program entry point
func main()
{
	rpio.Open()

	//loop indefinitely
	for i := 0 ; i ++ 
	{
		//check every 500ms
		time.Sleep(time.Millisecond * 500)
		
		//if barbell is off rack
		if rack.Read()
		{
			//execute main program
			run()
		}
	}
}


//main program loop
func run() 
{
	//arbitraru height variable
	height := 50	
	//main message channel with buffer 10 to avoid "deadlock"
	c := make(chan string, 10)

	go updateHeight(&height, c)
	go checkTime(&height, c)
	go checkFallen(&height, c)
	go checkHelp(c) 
	go checkRack(c)

	//iterate over channel range while it is open
	for msg := range c
	{
		//if barbell has fallen	
		if msg == "fall" 
		{
			//reracK barbell

			//kill unnecessary channels
			
			return
		}

		//if reracked
		if msg == "rack"
		{

			return 
		}

		//if lifter is struggling
		if msg == "stuggle"
		{
			//send message to checkHelp()
		}
	}
}

//update pointer to height variable
func updateHeight(height *int, c chan string)
{

}

//look at the rep times for signs of struggle
func checkTime(height *int, c chan string)
{

}

//check if the barbell has fallen 
func checkFallen(height *int, c chan string)
{

}	

//look at the help button
func checkHelp(c chan string)
{
	c <- "struggle"
}

//check if barbell has be reracked
func checkRack(c chan string)
{
	c <- "rack"
}