package main

import (
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
	"time"
)

func main() {
	size := 50
	bar2 := progressbar.Default(int64(size), "Bar2")

	for i := 0; i < size-1; i++ {
		//bar.Clear()
		//fmt.Println("writing meantime")
		//fmt.Println("writing more stuff..")

		bar2.Add(1)
		time.Sleep(100 * time.Millisecond)
	}

	bar2.Clear()
	(&zerolog.Logger{}).Info().Msg("writing more stuff..")
	//fmt.Println("writing more stuff..")


	current := bar2.GetMax()
	bar2.Clear()
	bar2.Reset()
	bar2.ChangeMax(size*2)
	bar2.Set(current)
	for i := 0; i < size; i++ {

		bar2.Add(1)
		time.Sleep(1000 * time.Millisecond)
	}

	//bar.Finish()
}
