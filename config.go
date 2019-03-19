package gorpc

import "path/filepath"
var logDir = filepath.Join("./","log")
var LogFilename = "rpclog"
var DebugLevel = "debug"
func loadConfig(){
	initLogRotator(filepath.Join(logDir,LogFilename))
	setLogLevels(DebugLevel)
}
