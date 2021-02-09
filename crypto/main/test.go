package main

import (
	"fmt"
	"time"
)

func main() {

	c1 := make(chan string, 1)
	c2 := make(chan string, 2)
	go func() {
		time.Sleep(500 * time.Millisecond)
		c1 <- "result 1"
		c2 <- "result 2"
		c2 <- "result 3"
	}()

	//c2 <- "result 2"
	//c2 <- "result 3"

	for {
		select {
		case res := <-c1:
			fmt.Println(res)
		case <-time.After(1 * time.Second):
			fmt.Println("timeout 1")
			/*for len(c2) != 0 {
				fmt.Println(len(c2))
				m := <-c2
				c1 <- m
			}*/
			m := <-c2
			c1 <- m
			//m = <-c2
			//c1 <- m
		}
	}
}
