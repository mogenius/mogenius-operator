package manager

import (
	"fmt"
	"time"
)

func Init() {
	for i := 0; i < 1000000; i++ {
		fmt.Println(i)
		time.Sleep(1 * time.Second)
	}
}
