package main

import (
	"errors"
	"fmt"
	"log"

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
	logHandler.Debug("ipam --->GetCapabilities")
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
	if len(req.Pool) == 0 {
		return nil, errors.New("subnet has invalid CIDR addr")
	}

	uuidStr := uuid.NewV4().String()
	logHandler.Debug("generate uuid ===>: %v", uuidStr)
	ipnet, err := ParseCIDR(req.Pool)
	if err != nil {
		logHandler.Debug("parse cidr [%v] fail:%v", req.Pool, err)
		return nil, err
	}

	PoolManager.lock()
	defer PoolManager.unlock()
	netpool := NewNetPool()
	netpool.get()
	netpool.Subnet = ipnet

	netpool.lowIp = ipAdd(ipnet.IP, 1)
	netpool.maxIp = getMaxIP(ipnet)
	PoolManager.Pools[uuidStr] = *netpool
	logHandler.Debug("subnet:%v,lowestip:%v,highest%v，", netpool.Subnet.String(), netpool.lowIp.String(), netpool.maxIp.String())

	//Pool必须是一个CIDR地址
	return &ipam.RequestPoolResponse{PoolID: uuidStr, Pool: ipnet.String()}, nil
}

func (aIpam *AppnetIpam) ReleasePool(req *ipam.ReleasePoolRequest) error {
	logHandler.Debug("ipam -->ReleasePool")
	logHandler.Debug("%v", *req)

	PoolManager.lock()
	defer PoolManager.unlock()

	logHandler.Debug("start to release pool [%v]", req.PoolID)
	err := PoolManager.ReleasePool(req.PoolID)
	if err != nil {
		logHandler.Error("release pool fail:%v", err)
		return fmt.Errorf("release pool fail:%v", err)
	}

	return nil
}

func (aIpam *AppnetIpam) RequestAddress(req *ipam.RequestAddressRequest) (*ipam.RequestAddressResponse, error) {
	logHandler.Debug("ipam --->RequestAddress")
	logHandler.Debug("%v", *req)

	//必须是CIDR地址
	PoolManager.lock()
	defer PoolManager.unlock()
	pool, exists := PoolManager.Pools[req.PoolID]
	if !exists {
		return nil, errors.New("pool %v doesn't exist")
	}

	var addr string
	var err error
	for k, v := range req.Options {
		switch k {
		case "RequestAddressType":
			if v == "com.docker.network.gateway" {
				logHandler.Debug("start to create gateway address")
				addr, err = pool.GetGateway(pool.Subnet)
				if err != nil {
					return nil, err
				}
				logHandler.Debug("gateway ip :%v", addr)
				return &ipam.RequestAddressResponse{
					Address: addr,
				}, nil
			}
			//这里这个ip地址应该怎么处理？？
			//要处理mac地址的问题.不同容器使用了同一个mac地址，ip不同会出现问题。
			//会出现mac地址相同的问题吗？
			//暂时先不考虑
		case "com.docker.network.endpoint.macaddres":
			//已分配ip地址的mac地址不再分配ip
			logHandler.Debug("check if mac address [%v] has used", v)
			_, exists := PoolManager.macMapping[v]
			if exists {
				return nil, fmt.Errorf("mac addr [%v] has already got an ip", v)
			}

			ipaddr, err := pool.CreateNewAddress(req.Address)
			if err != nil {
				return nil, err
			}

			//记录mac地址映射的ip
			PoolManager.macMapping[v] = ipaddr
			pool.get()
			return &ipam.RequestAddressResponse{
				Address: ipaddr.String(),
			}, nil

		default:
			logHandler.Debug("unhandler reqOption %v[%v]", k, v)

		}
	}

	//
	return nil, nil
}

func (aIpam *AppnetIpam) ReleaseAddress(req *ipam.ReleaseAddressRequest) error {
	logHandler.Debug("ipam ---> ReleaseAddress")
	logHandler.Debug("%v", *req)
	PoolManager.lock()
	defer PoolManager.unlock()

	pool, exists := PoolManager.Pools[req.PoolID]
	if !exists {
		logHandler.Debug("pool[%v] doesn't exists", req.PoolID)
		return fmt.Errorf("pool doesn't exists")
	}

	logHandler.Debug("start to release address [%v]", req.Address)
	ip, err := pool.ReleaseAddress(req.Address)
	if err != nil {
		return fmt.Errorf("release address fail for %v", err)
	}

	logHandler.Debug("start to clean mac-ip mapping")
	for k, v := range PoolManager.macMapping {
		logHandler.Debug("macMap[%v](%v) <===> %v", k, v.String(), ip.String())
		if v.String() == ip.String() {
			delete(PoolManager.macMapping, k)
			return nil
		}
	}

	pool.put()
	logHandler.Warn("bug: ip doesn't mapped to mac addr")
	return nil
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

	h := ipam.NewHandler(aIpam)

	logHandler.Debug("serve 9527 ...")
	err := h.ServeTCP("appnet", ":9527")
	if err != nil {
		logHandler.Error("%v", err)
	}
}
