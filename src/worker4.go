package main

import (
	"fmt"
	"os"
	"strconv"
	"tools"
	"worker"
)

func main() {
	fmt.Println("start")
	fmt.Printf("%v-----\n", os.Args[0])
	fmt.Printf("%v-----\n", os.Args[1])  //workerID
	fmt.Printf("%v-----\n", os.Args[2])  //partitionNum
	fmt.Printf("%v-----\n", os.Args[3])  //partitionstrategy
	workerID, err := strconv.Atoi(os.Args[1])
	PartitionNum, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("conv fail!")
	}
	tools.SetGraph(os.Args[3])

	worker.RunCCWorker(workerID, PartitionNum)
	fmt.Println("stop")
}