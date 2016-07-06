package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/astaxie/beego/logs"
)

var (
	logHandler *logs.BeeLogger
)

func initLogger(logfile string) error {

	fileCreate := true
	if err := os.MkdirAll(filepath.Dir(logfile), 0755); err != nil {
		if !os.IsExist(err) {
			fileCreate = false
		} else {
			if _, err := os.Create(logfile); err != nil {
				if !os.IsExist(err) {
					fileCreate = false
				}
			}
		}
	}

	if !fileCreate {
		fmt.Println("logfile fail")
		return fmt.Errorf("can't create log file")
	}

	log := logs.NewLogger(100000)
	log.SetLogger("console", "")
	log.SetLevel(logs.LevelDebug)
	log.EnableFuncCallDepth(true)

	jsonConfig := fmt.Sprintf("{\"filename\":\"%s\",\"maxlines\":5000,\"maxsize\":10240000}", logfile)
	log.SetLogger("file", jsonConfig)
	logHandler = log
	return nil
}
