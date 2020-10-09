package tools

const (
	ResultPath = "/mnt/sdb1/zhangshuai/result/"
	//dataPath = "/slurm/zhangshuai/lj_4/"
	dataPath = "/slurm/zhangshuai/"
	WorkerOnSC = false

	//PatternPath = "../test_data/pattern.txt"
	PatternPath = "pattern.txt"
	GraphSimulationTypeModel = 100

	RPCSendSize = 100000

	ConnPoolSize = 16
	MasterConnPoolSize = 1024
)

var partitionStrategy string
var graphName string

func SetGraph(strategy string, graph string) {
	partitionStrategy = strategy
	graphName = graph
}

func GetConfigPath(partitionNum int) string {
	if partitionNum == 12 {
		return "config12.txt"
	} else {
		return "config32.txt"
	}
	//return "config4.txt"
}

func GetNFSPath() string {
	return dataPath + graphName + "_" + partitionStrategy + "/"
	//return dataPath + partitionStrategy + "/"
}