package shared

func InitCore() {
	initLogger()
	initConfig()
	initK8s()
	initHelm()
}
