package shared

func InitCore() {
	initConfig()
	initLogger()
	initK8s()
	initHelm()
	initDocker()
}
