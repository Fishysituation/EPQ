package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {

	file := create("logs/output")
	write("Hello, wo!", file)
}

func write(text string, file *os.File) {

	writer := bufio.NewWriter(file)
	defer file.Close()

	fmt.Fprintln(writer, text)
	writer.Flush()
}

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
