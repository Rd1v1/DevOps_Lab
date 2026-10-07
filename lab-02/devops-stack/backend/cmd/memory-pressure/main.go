// Used only by the diagnostic Docker target to reproduce a cgroup OOM.
package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	chunks := make([][]byte, 0)
	for {
		chunk := make([]byte, 8<<20)
		for i := range chunk {
			chunk[i] = 1
		}
		chunks = append(chunks, chunk)
		fmt.Printf("allocated %d MiB\n", len(chunks)*8)
		runtime.KeepAlive(chunks)
		time.Sleep(200 * time.Millisecond)
	}
}
