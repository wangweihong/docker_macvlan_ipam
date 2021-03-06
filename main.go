package main

import (
	"errors"
	"fmt"
	"net"
	"time"

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

	netpool.LowIp = ipAdd(ipnet.IP, 1)
	netpool.MaxIp = getMaxIP(ipnet)
	PoolManager.Pools[uuidStr] = *netpool
	logHandler.Debug("subnet:%v,lowestip:%v,highest%v，", netpool.Subnet.String(), netpool.LowIp.String(), netpool.MaxIp.String())

	//这里需要完善备份失败后,数据应当如何还原
	logHandler.Debug("PoolManager:%v", PoolManager)
	err = BackendClient.Save(PoolManager)
	if err != nil {
		logHandler.Debug("Save fail:%v", err)
		logHandler.Debug("backend save fail")
		return nil, err
	}

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

	//这里需要完善备份失败后,数据应当如何还原
	err = BackendClient.Save(PoolManager)
	if err != nil {
		logHandler.Debug("Save fail:%v", err)
		return err
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
		return nil, fmt.Errorf("pool %v doesn't exist", req.PoolID)
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

				//这里需要完善备份失败后,数据应当如何还原
				logHandler.Debug("PoolManager:%v", PoolManager)
				err = BackendClient.Save(PoolManager)
				if err != nil {
					logHandler.Debug("Save fail:%v", err)
					ip, err1 := pool.ReleaseAddress(addr)
					if err1 != nil {
						logHandler.Debug("release address[%v] fail:%v", ip.String(), err.Error())
					}
					pool.Gateway = ""
					logHandler.Debug("backend save fail")
					return nil, err
				}

				return &ipam.RequestAddressResponse{
					Address: addr,
				}, nil
			}
			//这里这个ip地址应该怎么处理？？
			//要处理mac地址的问题.不同容器使用了同一个mac地址，ip不同会出现问题。
			//会出现mac地址相同的问题吗？
			//暂时先不考虑
		case "com.docker.network.endpoint.macaddress":
			//已分配ip地址的mac地址不再分配ip
			logHandler.Debug("check if mac address [%v] has used", v)
			_, exists := PoolManager.MacMapping[v]
			if exists {
				return nil, fmt.Errorf("mac addr [%v] has already got an ip", v)
			}

			logHandler.Debug("start tot create new address")
			ipaddr, err := pool.CreateNewAddress(req.Address)
			if err != nil {
				return nil, err
			}

			//记录mac地址映射的ip
			PoolManager.MacMapping[v] = ipaddr

			iNet := net.IPNet{IP: ipaddr, Mask: pool.Subnet.Mask}

			//这里需要完善备份失败后,数据应当如何还原
			logHandler.Debug("PoolManager:%v", PoolManager)
			err = BackendClient.Save(PoolManager)
			if err != nil {
				logHandler.Debug("Save fail:%v", err)
				ip, err1 := pool.ReleaseAddress(addr)
				if err1 != nil {
					logHandler.Debug("release address[%v] fail:%v", ip.String(), err.Error())
				}
				delete(PoolManager.MacMapping, v)
				logHandler.Debug("backend save fail")
				return nil, err
			}

			//返回的是CIDR地址
			return &ipam.RequestAddressResponse{Address: iNet.String()}, nil

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
	for k, v := range PoolManager.MacMapping {
		logHandler.Debug("macMap[%v](%v) <===> %v", k, v.String(), ip.String())
		if v.String() == ip.String() {
			delete(PoolManager.MacMapping, k)
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

func syncBackend() (*NetPools, error) {
	for {
		//	BackendClient.HealthCheck()
		logHandler.Debug("start to init pool manager")
		pools := InitNetPools()
		logHandler.Debug(" pool manager:%v\n", pools)
		err := BackendClient.Get(pools)
		if err != nil {
			//可能是第一次启动
			if err.Error() == ErrBackendKeyNotFound.Error() {
				return pools, nil
			}
			logHandler.Error("sync BackendClient fail:%v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		return pools, nil
	}
}

func main() {
	config := LoadConfig()
	if len(config.BackendUrl) == 0 {
		panic("ipam must set backend url for backend store")
	}

	initLogger("./ipam.log")
	logHandler.Debug("create appnet ipam handler...")

	if os.Getenv("APPNET_DEBUG") != "true" {
		logHandler.Debug("%v", os.Getenv("APPNET_DEBUG"))
		logHandler.Info("close debuging ..")
		CloseDebug()
	}

	//待完成:同步备份的数据
	//待完成2:后端数据库崩溃后,所有请求的失败处理
	//待完成3:大量ip地址备份时,所需要的时间开销测试。以此来调整后端存储的格式。
	bd := NewBackend(config.BackendUrl)
	if bd == nil {
		panic("create ipam backend client fail")
	}
	IpamBackend = bd
	go IpamBackend.HealthCheck()

	//避免后端存储仍未启动
	var count int
	for {
		if !IpamBackend.Alive() {
			if count == 0 {
				logHandler.Debug("backend haven't start yet, wait..")
			}
			count += 1
			if count == 20 {
				count = 0
			}
			time.Sleep(3 * time.Second)
			continue
		}

		logHandler.Debug("backend have start")
		break
	}

	BackendClient = IpamBackend

	var err error
	PoolManager, err = syncBackend()
	if err != nil {
		logHandler.Error("sync backend fail for %v", err)
		os.Exit(1)
	}
	logHandler.Debug("PoolMangger:%v", PoolManager)
	syncIpMap()

	//这里还需要进行backend和内存中的同步

	aIpam := NewAppnetIpam()

	h := ipam.NewHandler(aIpam)

	logHandler.Debug("serve 9527 ...")
	err = h.ServeTCP("appnet", ":9527")
	if err != nil {
		logHandler.Error("%v", err)
	}
}
