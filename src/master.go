package main

import (
	"bufio"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"os"
	pb "protobuf"
	"strconv"
	"strings"
	"sync"
	"time"
	"tools"
)

type Master struct {
	mu      *sync.Mutex
	spmu    *sync.Mutex
	newCond *sync.Cond
	address string
	// signals when Register() adds to workers[]
	// number of worker
	workerNum      int
	workersAddress []string
	//master know worker's address from config.txt and worker's index
	//TCP connection Reuse : after accept resgister then dial and save the handler
	//https://github.com/grpc/grpc-go/issues/682
	isWorkerRegistered map[int32]bool
	//each worker's RPC address
	//TODO: split subgraphJson into worker's partition
	subGraphFiles string
	// Name of Input File subgraph json
	partitionFiles string
	// Name of Input File partition json
	shutdown     chan struct{}
	registerDone chan bool
	statistic    []int32
	//each worker's statistic data
	wg          *sync.WaitGroup
	JobDoneChan chan bool

	finishMap map[int32]bool
	finishDone chan bool

	allSuperStepFinish bool

	totalIteration int64

	workerConn map[int]*grpc.ClientConn

	//calTime map[int32]float64
	//sendTime map[int32]float64
}

func (mr *Master) Lock() {
	mr.mu.Lock()
}

func (mr *Master) Unlock() {
	mr.mu.Unlock()
}

func (mr *Master) SPLock() {
	mr.spmu.Lock()
}

func (mr *Master) SPUnlock() {
	mr.spmu.Unlock()
}

// Register is an RPC method that is called by workers after they have started
// up to report that they are ready to receive tasks.
// Locks for multiple worker concurrently access worker list
func (mr *Master) Register(ctx context.Context, args *pb.RegisterRequest) (r *pb.RegisterResponse, err error) {
	mr.Lock()
	defer mr.Unlock()

	log.Printf("Register: worker %d\n", args.WorkerIndex)
	endpoint := mr.workersAddress[args.WorkerIndex]
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	mr.workerConn[int(args.WorkerIndex)] = conn

	if _, ok := mr.isWorkerRegistered[args.WorkerIndex]; ok {
		log.Fatal("%d worker register more than one times", args.WorkerIndex)
	} else {
		mr.isWorkerRegistered[args.WorkerIndex] = true
	}
	if len(mr.isWorkerRegistered) == mr.workerNum {
		mr.registerDone <- true
	}
	log.Printf("len:%d\n", len(mr.isWorkerRegistered))
	log.Printf("workernum:%d\n", mr.workerNum)
	// There is no need about scheduler
	return &pb.RegisterResponse{Ok: true}, nil
}

// newMaster initializes a new Master
func newMaster() (mr *Master) {
	mr = new(Master)
	mr.shutdown = make(chan struct{})
	mr.mu = new(sync.Mutex)
	mr.spmu = new(sync.Mutex)
	mr.newCond = sync.NewCond(mr.mu)
	mr.JobDoneChan = make(chan bool)
	mr.registerDone = make(chan bool)
	mr.wg = new(sync.WaitGroup)
	//workersAddress slice's index is worker's Index
	//read from Config text
	mr.workersAddress = make([]string, 0)
	mr.isWorkerRegistered = make(map[int32]bool)
	mr.statistic = make([]int32, mr.workerNum)
	mr.totalIteration = 0

	mr.finishDone = make(chan bool)
	mr.finishMap = make(map[int32]bool)

	mr.workerConn = make(map[int]*grpc.ClientConn)
	//mr.calTime = make(map[int32]float64)
	//mr.sendTime = make(map[int32]float64)
	return mr
}
func (mr *Master) ReadConfig(partitionNum int) {
	configPath := tools.GetConfigPath(partitionNum)
	f, err := os.Open(configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	first := true
	for {
		line, err := rd.ReadString('\n')
		line = strings.Split(line, "\n")[0] //delete the end "\n"
		if err != nil || io.EOF == err {
			break
		}
		conf := strings.Split(line, ",")
		//TODO: this operate is for out of range
		if first {
			mr.workersAddress = append(mr.workersAddress, "0")
			mr.address = conf[1]
			log.Print(mr.address)
			first = false
		} else {
			mr.workersAddress = append(mr.workersAddress, conf[1])
			log.Print(conf[1])
		}
	}
	mr.workerNum = len(mr.workersAddress) - 1
}
func (mr *Master) wait() {
	<-mr.JobDoneChan
}
func (mr *Master) KillWorkers() {
	log.Println("start master kill")

	batch := (mr.workerNum + tools.MasterConnPoolSize - 1) / tools.MasterConnPoolSize

	for i := 1; i <= batch; i++ {
		for j := (i - 1) * tools.MasterConnPoolSize + 1; j <= mr.workerNum && j <= i * tools.MasterConnPoolSize; j++ {
			mr.wg.Add(1)
			log.Printf("Master: shutdown worker %d\n", j)

			go func(id int) {
				defer mr.wg.Done()

				/*endpoint := mr.workersAddress[id]
				conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
				if err != nil {
					panic(err)
				}
				defer conn.Close()
*/
				handler := mr.workerConn[id]
				client := pb.NewWorkerClient(handler)
				shutDownReq := &pb.ShutDownRequest{}
				client.ShutDown(context.Background(), shutDownReq)

			}(j)
		}
		mr.wg.Wait()
	}
}
func (mr *Master) StartMasterServer() {
	grpcServer := grpc.NewServer()
	pb.RegisterMasterServer(grpcServer, mr)

	ln, err := net.Listen("tcp", mr.address)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := grpcServer.Serve(ln); err != nil {
			panic(err)
		}
	}()
}

func (mr *Master) PEval() bool {
	batch := (mr.workerNum + tools.MasterConnPoolSize - 1) / tools.MasterConnPoolSize

	for i := 1; i <= batch; i++ {
		for j := (i-1)*tools.MasterConnPoolSize + 1; j <= mr.workerNum && j <= i*tools.MasterConnPoolSize; j++ {
			log.Printf("Master: start %d PEval", j)
			mr.wg.Add(1)
			go func(id int) {
				defer mr.wg.Done()

				handler := mr.workerConn[id]
				client := pb.NewWorkerClient(handler)
				//pevalRequest := &pb.PEvalRequest{}
				if pevalResponse, err := client.PEval(context.Background(), &pb.PEvalRequest{}); err != nil {
					log.Printf("Fail to execute PEval %d\n", id)
					log.Fatal(err)
					//TODO: still something todo: Master Just terminate, how about the Worker
				} else if !pevalResponse.Ok {
					log.Printf("This worker %v dosen't participate in this round\n!", id)
				}
			}(j)
		}

		mr.wg.Wait()
	}
	return true
}

func (mr *Master) MessageExchange() bool {
	batch := (mr.workerNum + tools.MasterConnPoolSize - 1) / tools.MasterConnPoolSize

	for i := 1; i <= batch; i++ {
		for j := (i-1)*tools.MasterConnPoolSize + 1; j <= mr.workerNum && j <= i*tools.MasterConnPoolSize; j++ {
			log.Printf("Master: start %d MessageExchange", j)
			mr.wg.Add(1)
			go func(id int) {
				defer mr.wg.Done()

				handler := mr.workerConn[id]
				client := pb.NewWorkerClient(handler)
				//pevalRequest := &pb.PEvalRequest{}
				if exchangeResponse, err := client.ExchangeMessage(context.Background(), &pb.ExchangeRequest{}); err != nil {
					log.Printf("Fail to execute Exchange %d\n", id)
					log.Fatal(err)
				} else if !exchangeResponse.Ok {
					log.Printf("This worker %v dosen't participate in this round\n!", id)
				}
			}(j)
		}
		mr.wg.Wait()
	}
	return true
}

func (mr *Master) SuperStepFinish(ctx context.Context, args *pb.FinishRequest) (r *pb.FinishResponse, err error) {
	mr.SPLock()
	defer mr.SPUnlock()

	//mr.calTime[args.WorkerID] += args.IterationSeconds
	//mr.sendTime[args.WorkerID] += args.AllPeerSend

	//log.Printf("combine seconds:%v\n", args.CombineSeconds)

	if args.CombineSeconds >= 0 {
		mr.finishMap[args.WorkerID] = args.MessageToSend
		mr.allSuperStepFinish = mr.allSuperStepFinish || args.MessageToSend
		if len(mr.finishMap) == mr.workerNum {
			mr.finishDone <- mr.allSuperStepFinish
		}

		log.Printf("zs-log: message to send:%v\n", args.MessageToSend)

		log.Printf("worker %v IterationNum : %v\n", args.WorkerID, args.IterationNum)
		log.Printf("worker %v duration time of calculation: %v\n", args.WorkerID, args.IterationSeconds)
		log.Printf("worker %v number of updated boarders node pair : %v\n", args.WorkerID, args.UpdatePairNum)
		log.Printf("worker %v number of destinations which message send to: %v\n", args.WorkerID, args.DstPartitionNum)
		log.Printf("worker %v duration of send message to all other workers : %v\n", args.WorkerID, args.AllPeerSend)
		for _, pairNum := range args.PairNum {
			log.Printf("worker %v send to worker %v %v messages\n", args.WorkerID, pairNum.WorkerID, pairNum.CommunicationSize)
		}

		mr.totalIteration += args.IterationNum
		log.Printf("iteration num:%v\n", mr.totalIteration)
	} else {
		log.Printf("worker %v duration time of calculation in exchange: %v\n", args.WorkerID, args.IterationSeconds)
		log.Printf("worker %v duration of send message to all other workers in exchange: %v\n", args.WorkerID, args.AllPeerSend)
	}
	return &pb.FinishResponse{Ok: true}, nil
}

func (mr *Master) CalculateFinish(ctx context.Context, args *pb.CalculateFinishRequest) (r *pb.CalculateFinishResponse, err error) {
	mr.Lock()
	defer mr.Unlock()

/*	mr.finishMap[args.WorkerIndex] = true

	if len(mr.finishMap) == mr.workerNum {
		mr.finishDone <- true
	}
*/
	return &pb.CalculateFinishResponse{Ok:true}, nil
}


// this function is the set of all incremental superstep done
func (mr *Master) IncEval(step int) bool {
	batch := (mr.workerNum + tools.MasterConnPoolSize - 1) / tools.MasterConnPoolSize
	for i := 1; i <= batch; i++ {
		for j := (i-1)*tools.MasterConnPoolSize + 1; j <= mr.workerNum && j <= i*tools.MasterConnPoolSize; j++ {
			log.Printf("Master: start the %vth Incval of worker %v", step, j)
			mr.wg.Add(1)
			go func(id int) {
				defer mr.wg.Done()
/*
				endpoint := mr.workersAddress[id]
				conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
				if err != nil {
					panic(err)
				}
				defer conn.Close()
*/
				handler := mr.workerConn[id]

				client := pb.NewWorkerClient(handler)
				incEvalRequest := &pb.IncEvalRequest{}
				if reply, err := client.IncEval(context.Background(), incEvalRequest); err != nil {
					log.Fatalf("Fail to execute IncEval worker %v", id)
				} else if !reply.Update {
					log.Printf("This worker %v dosen't update in the round %v\n!", id, step)
				}
			}(j)
		}
		mr.wg.Wait()
	}
	return true
}

func (mr *Master) Assemble() bool {
	batch := (mr.workerNum + tools.MasterConnPoolSize - 1) / tools.MasterConnPoolSize
	for i := 1; i <= batch; i++ {
		for j := (i-1)*tools.MasterConnPoolSize + 1; j <= mr.workerNum && j <= i*tools.MasterConnPoolSize; j++ {
			log.Printf("Master: start worker %v Assemble", j)
			mr.wg.Add(1)
			go func(id int) {
				defer mr.wg.Done()
				handler := mr.workerConn[id]

				client := pb.NewWorkerClient(handler)
				assembleRequest := &pb.AssembleRequest{}
				if _, err := client.Assemble(context.Background(), assembleRequest); err != nil {
					log.Fatal("Fail to execute Assemble worker %v", id)
				}
			}(j)
		}
		mr.wg.Wait()
	}
	return true
}
func (mr *Master) StopRPCServer() {
}

func (mr *Master) ClearSuperStepMessgae() {
	mr.allSuperStepFinish = false
	mr.finishMap = make(map[int32]bool)
}

func RunJob(jobName string, workerNum int) {
	log.Println(jobName)
	mr := newMaster()
	mr.ReadConfig(workerNum)
	go mr.StartMasterServer()
	<-mr.registerDone

	log.Println("start PEval")
	start := time.Now()
	mr.ClearSuperStepMessgae()
	mr.PEval()
	<-mr.finishDone
	mr.MessageExchange()
	log.Println("end PEval")

	log.Println("start IncEval")
	step := 0
	for {
		step++
		mr.ClearSuperStepMessgae()
		mr.IncEval(step)
		finish :=<- mr.finishDone
	//	log.Printf("finish: %v\n", finish)
		if !finish {
			break
		}
		mr.MessageExchange()
	}
	log.Println("end IncEval")

	runTime := time.Since(start)

	log.Printf("runTime: %vs\n", runTime.Seconds())
	/*var i int32
	for i = 1; i <= int32(mr.workerNum); i++ {
		log.Printf("worker %v calculate time:%v, send message time: %v, waiting time: %v\n", i, mr.calTime[i], mr.sendTime[i], runTime.Seconds() - mr.calTime[i] - mr.sendTime[i])
	}*/
	//fmt.Printf("teps:%v\n", float64(mr.totalIteration) / runTime.Seconds())
	log.Printf("teps:%v\n", float64(mr.totalIteration) / runTime.Seconds())
	mr.Assemble()
	mr.KillWorkers()
	mr.StopRPCServer()
	//mr.wait()
	log.Printf("Job finishes")
}

func main() {
	jobName := os.Args[1]
	workerNum, _ := strconv.Atoi(os.Args[2])

	//TODO:split the Json into worker's subJson
	RunJob(jobName, workerNum)
}
