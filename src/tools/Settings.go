package tools

const (
	ResultPath = "/mnt/sdb1/zhangshuai/result/"
	dataPath = "/slurm/zhangshuai/"

	WorkerOnSC = false

	//PatternPath = "../test_data/pattern.txt"
	PatternPath = "pattern.txt"
	GraphSimulationTypeModel = 100

	RPCSendSize = 100000

	ConnPoolSize = 16
	MasterConnPoolSize = 1024
)

var graphName, partitionStrategy string

func SetGraph(name string, strategy string) {
	graphName = name
	partitionStrategy = strategy
}

func GetConfigPath(partitionNum int) string {
	if partitionNum == 12 {
		return "config12.txt"
	} else {
		return "config32.txt"
	}
}

func GetNFSPath() string {
	return dataPath + graphName + "_" + partitionStrategy + "/"
}