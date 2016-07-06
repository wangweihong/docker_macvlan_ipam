package main

import (
	"log"
	"reflect"

	"os"

	"github.com/docker/go-plugins-helpers/ipam"
	uuid "github.com/satori/go.uuid"
)

//ipam driver for appnet
type AppnetIpam struct {
}

//激活插件
func (aIpam *AppnetIpam) PluginActivate(r interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Implements": []interface{}{
			"IpamDriver",
		}}, nil
}

func (aIpam *AppnetIpam) GetCapabilities() (*ipam.CapabilitiesResponse, error) {
	logHandler.Debug("iparm --->GetCapabilities")
	return &ipam.CapabilitiesResponse{RequiresMACAddress: true}, nil
}

func (aIpam *AppnetIpam) GetDefaultAddressSpaces() (*ipam.AddressSpacesResponse, error) {
	logHandler.Debug("ipam -->GetDefaultAddressSpaces")
	asp := ipam.AddressSpacesResponse{
		LocalDefaultAddressSpace:  "localAppnet",
		GlobalDefaultAddressSpace: "globalAppnet",
	}
	return &asp, nil
}

func (aIpam *AppnetIpam) RequestPool(req *ipam.RequestPoolRequest) (*ipam.RequestPoolResponse, error) {

	logHandler.Debug("ipam -->RequestPool")
	logHandler.Debug("%v", *req)
	//return nil, nil
	uuidStr := uuid.NewV4().String()

	//Pool必须是一个CIDR地址
	return &ipam.RequestPoolResponse{PoolID: uuidStr, Pool: req.Pool}, nil
}

func (aIpam *AppnetIpam) ReleasePool(req *ipam.ReleasePoolRequest) error {
	logHandler.Debug("ipam -->ReleaseAddressRequestPool")
	logHandler.Debug("%v", *req)
	return nil
}
func (aIpam *AppnetIpam) RequestAddress(req *ipam.RequestAddressRequest) (*ipam.RequestAddressResponse, error) {
	logHandler.Debug("ipam --->RequestAddress")
	logHandler.Debug("%v", *req)

	//必须是CIDR地址
	return &ipam.RequestAddressResponse{
		Address: "192.168.15.1/24",
	}, nil
}

func (aIpam *AppnetIpam) ReleaseAddress(req *ipam.ReleaseAddressRequest) error {
	logHandler.Debug("ipam ---> ReleaseAddress")
	logHandler.Debug("%v", *req)
	return nil
}

type ipamCall struct {
	url string
	f   func(r interface{}) (map[string]interface{}, error)
	t   reflect.Type
}

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

func setupSocket(pluginDir string, driverName string) string {
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		if !os.IsExist(err) {
			log.Panicf("Create Plugin Directory error:'%s'", err)
			os.Exit(1)
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
	//	config := LoadConfig()
	//	setupSocket(config.PluginDir, config.DriverName)

	initLogger("./ipam.log")
	logHandler.Debug("create appnet ipam handler...")
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

	logHandler.Debug("serve 9527 ...")
	err := h.ServeTCP("appnet", ":9527")
	if err != nil {
		logHandler.Error("%v", err)
	}
}
