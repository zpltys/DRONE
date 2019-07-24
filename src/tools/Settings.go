package tools


const (
	//ResultPath = "/home/acgict/result/"
	ResultPath = "/slurm/zhangshuai/result/"
	//ResultPath = "./"
	//NFSPath = "/home/xwen/graph/16/"
	NFSPath = "/slurm/zhangshuai/twitter_cdbh_32/"

	WorkerNum = 32
	WorkerOnSC = false

	LoadFromJson = false

	ConfigPath = "config.txt"
	//ConfigPath = "../test_data/config.txt"

	PatternPath = "pattern.txt"
	GraphSimulationTypeModel = 100

	RPCSendSize = 10000


	ConnPoolSize = 128
	MasterConnPoolSize = 1024
)