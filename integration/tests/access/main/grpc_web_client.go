package main

import (
	"bufio"
"fmt"
"net/http"
)

func main() {

	//script := "pub struct Point {\n       pub var x: Int\n       pub var y: Int\n\n       init(x: Int, y: Int) {\n         self.x = x\n         self.y = y\n       }\n     }\n     pub fun main(): [Point] {\n       return [Point(x: 1, y: 2), Point(x: 3, y: 4)]\n     }"

	req, err := http.NewRequest("GET", "http://localhost:8080/access.AccessAPI/Ping", nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("x-grpc-web", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36")
	req.Header.Set("content-type", "application/grpc-web+proto")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Referer", "http://localhost:8080/")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")


	req.Host = "localhost"
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	fmt.Println("Response status:", resp.Status)

	scanner := bufio.NewScanner(resp.Body)
	for i := 0; scanner.Scan() && i < 5; i++ {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

