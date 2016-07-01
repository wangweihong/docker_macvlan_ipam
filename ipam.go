package main

import (
	"log"

	"github.com/docker/go-plugins-helpers/ipam"
)

//ipam driver for appnet
type AppnetIpam struct {
}

//激活插件
/*
func (aIpam *AppnetIpam) PluginActivate(r interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Implements": []interface{}{
			"IpamDriver",
		}}, nil
}

func (aIpam *AppnetIpam) RequestAddress(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (aIpam *AppnetIpam) ReleaseAddress(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (aIpam *AppnetIpam) RequestPool(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (aIpam *AppnetIpam) ReleasePool(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (aIpam *AppnetIpam) GetCapabilities(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (aIpam *AppnetIpam) GetDefaultAddressSpaces(r interface{}) (map[string]interface{}, error) {
	return nil, nil
}


type ipamCall struct {
	url string
	f   func(r interface{}) (map[string]interface{}, error)
	t   reflect.Type
}
*/
func NewAppnetIpam() *AppnetIpam {
	return &AppnetIpam{}
}

func fileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

func deleteFile(filePath string) error {
	return os.Remove(filePath)
}

func setupSocker(pluginDir string, driverName string) string {
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		if !os.isExists(err) {
			log.Panicf("Create Plugin Directory error:'%s'", err)
			ox.Exit(1)
		}
	}

	sockerFile := pluginDir + "/" + driverName + ".sock"

	exists, err := fileExists(sockerFile)
	if err != nil {
		log.Panicf("Stat Socket File error: '%s'", err)
		os.Exit(1)
	}

	if exists {
		err = deleteFile(sockerFile)
		if err != nil {
			log.Panicf("Delete Socket File error: '%s'", err)
			os.Exit(1)
		}
		log.Panicf("Delete Socket File error: '%s'", err)
		os.Exit(1)
	}
	log.Printf("Deleted Old Socket File: '%s'", sockerFile)

	return sockerFile
}

func main() {
	config := LoadConfig()

	aIpam := NewAppnetIpam()
	/*
		ipamCalls := []ipamCalls{
			{"/Plugin.Activate", aIpam.PluginActivate, nil},
			{"/IpamDriver.GetCapabilities", aIpam.GetCapabilities, nil},
			{"/IpamDriver.GetDefaultAddressSpaces", aIpam.GetDefaultAddressSpaces, nil},
			{"/IpamDriver.RequestPool", aIpam.RequestPool, nil},
			{"/IpamDriver.ReleasePool", aIpam.ReleasePool, nil},
			{"/IpamDriver.RequestAddress", aIpam.RequestAddress, nil},
		{"/IpamDriver.ReleaseAddress", aIpam.ReleasePool, nil},
		}
	*/

	h := ipam.NewHandler(aIpam)
	h.ServeTCP("appnet", ":9527")
}
