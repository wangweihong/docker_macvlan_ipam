package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
)

var (
	PoolManager *NetPools
	IpamBackend Backend
)

type NetPools struct {
	Pools  map[string]NetPool `json:"pools"`
	rwlock *sync.RWMutex
	//macaddress:net.IP
	MacMapping map[string]net.IP `json:"macmapping"`
}

type NetPool struct {
	Reference int       `json:"reference"`
	Subnet    net.IPNet `json:"subnet"`
	Gateway   string    `json:"gateway"`
	//记录当前池使用了哪些网络 key:"192.168.0.1"
	IPs   map[string]net.IP `json:"ips"`    //value:转换后的数值
	LowIp net.IP            `json:"lowip"`  //最低可用IP转换成数值
	MaxIp net.IP            `json:"highip"` //最高可以用IP转换数值
}

func NewNetPool() *NetPool {
	netpool := new(NetPool)
	netpool.IPs = make(map[string]net.IP)
	return netpool
}

func (n *NetPools) lock() {
	logHandler.Debug("lock...")
	n.rwlock.Lock()
}

func (n *NetPools) unlock() {
	logHandler.Debug("unlock...")
	n.rwlock.Unlock()
}

func (n *NetPool) get() {
	n.Reference += 1
}
func (n *NetPool) put() {
	n.Reference -= 1
}

//数组逆序
func sliceReverse(a []byte) {
	for left, right := 0, len(a)-1; left < right; left, right = left+1, right-1 {
		a[left], a[right] = a[right], a[left]
	}
}

//排除.0或者.255两个IP地址
func isvalidIP(originIp net.IP) bool {
	logHandler.Debug("originIp:%v, bytes format ==> %v", originIp.String(), originIp)
	lastByte := originIp[len(originIp)-1]
	logHandler.Debug("last Byte is %v", lastByte)
	if lastByte == uint8(0) || lastByte == uint8(255) {
		return false
	}

	return true
}

// 1 大于
// 0 相等
//-1 小于
func compareIP(aIP, bIP net.IP) (int, error) {
	return 0, nil
}

//将ip地址转换成数字
//0 - 意味着无效的Ip
func IPtoNum(originIp net.IP) uint32 {
	logHandler.Debug("net.IP len : %v", len(originIp))
	ip := make([]byte, len(originIp))
	copy(ip, originIp)

	a := []byte{}
	if len(ip) == 16 {
		for i := 12; i < 16; i++ {
			a = append(a, ip[i])
		}
	} else {
		for i := 0; i < 4; i++ {
			a = append(a, ip[i])
		}
	}

	logHandler.Debug("before:%v", a)

	//逆序
	sliceReverse(a)
	logHandler.Debug("reverse:%v", a)

	//only  support little endian operation
	//转换成整数
	data := binary.LittleEndian.Uint32(a)
	sliceReverse(a)
	logHandler.Debug("back:%v", a)

	return data
}

//获得subnet中允许的最大ip
func getMaxIP(ipnet net.IPNet) net.IP {
	n := len(ipnet.IP)
	a := make([]byte, n)

	for i := 0; i < n; i++ {
		a[i] = ipnet.IP[i] | ^ipnet.Mask[i]
	}

	return net.IP(a)
}

//通过数值增加来添加ip
func ipAdd(originIp net.IP, num uint32) net.IP {
	if len(originIp) == 0 {
		logHandler.Debug("originIp is empty")
		return net.IP{}
	}
	logHandler.Debug("net.IP :%v,len: %v, num:%v", originIp, len(originIp), num)
	ip := make([]byte, len(originIp))
	copy(ip, originIp)

	a := []byte{}

	if len(ip) == 16 {
		for i := 12; i < 16; i++ {
			a = append(a, ip[i])
		}
	} else {
		for i := 0; i < 4; i++ {
			a = append(a, ip[i])
		}
	}

	logHandler.Debug("origin:%v", a)

	//逆序,需要逆序,如果是a["192","168","0","1"]==> a["1","0","168","192"]
	//不然算起来会加到192上
	sliceReverse(a)

	logHandler.Debug("reverse:%v", a)
	//only  support little endian operation
	//转换成整数
	data := binary.LittleEndian.Uint32(a)

	data += num

	//转换回[]byte
	binary.LittleEndian.PutUint32(a, data)
	//转换回正常顺序
	sliceReverse(a)
	logHandler.Debug("reverse again:%v", a)

	if len(ip) == 16 {
		for i := 0; i < 4; i++ {
			ip[i+12] = a[i]
		}
	} else {
		for i := 0; i < 4; i++ {
			ip[i] = a[i]
		}
	}

	return ip
}

//获取网关地址
func (pool *NetPool) GetGateway(ipnet net.IPNet) (string, error) {
	if len(pool.Gateway) != 0 {
		return pool.Gateway, nil
	}

	start := IPtoNum(pool.LowIp)
	end := IPtoNum(pool.MaxIp)

	//找到一个可用的IP地址
	var GatewayIP net.IP
	for {
		index, err := appnetIPMap.FindLowestUnsetBit(uint64(start), uint64(end))
		if err != nil {
			return "", err
		}

		start = uint32(index)
		offset := index - uint64(IPtoNum(ipnet.IP))
		logHandler.Debug("index:%v", index)
		GatewayIP = ipAdd(ipnet.IP, uint32(offset))

		logHandler.Debug("check gateway ip %v is valid", GatewayIP.String())
		if !isvalidIP(GatewayIP) {
			logHandler.Debug("is not avail")
			//尝试对无效ip写1,避免以后被使用
			if !appnetIPMap.SetBit(index, 1) {
				//出错就忽视
				logHandler.Warn("try to set [%v] in ipbitmap fail", GatewayIP)
			}
			logHandler.Debug("get an invalid ip , skip, try next")

		} else { //设置指定ip为已使用
			logHandler.Debug("is  avail")
			if !appnetIPMap.SetBit(index, 1) {

				logHandler.Error("try to set [%v] in ipbitmap fail", GatewayIP)
				return "", errors.New("save ip address fail")
			}
			break

		}
	}

	logHandler.Debug("Gateway:%v", GatewayIP.String())
	pool.Gateway = GatewayIP.String()
	logHandler.Debug("pool gate way :%v", pool.Gateway)
	pool.IPs[pool.Gateway] = GatewayIP
	newIpnet := net.IPNet{
		IP:   GatewayIP,
		Mask: ipnet.Mask,
	}

	return newIpnet.String(), nil

}

func (pool *NetPool) CreateNewAddress(reqAddr string) (net.IP, error) {
	//指定了要创建的ip地址
	if len(reqAddr) != 0 {
		reqIP := net.ParseIP(reqAddr)
		if reqIP == nil {
			return net.IP{}, fmt.Errorf("invalid request ip address \"%v\"", reqAddr)
		}

		reqIndex := IPtoNum(reqIP)

		if appnetIPMap.GetBit(uint64(reqIndex)) == uint8(1) {
			return net.IP{}, fmt.Errorf("request ip \"%v\"has beed used", reqAddr)
		} else {
			if !isvalidIP(reqIP) {
				return net.IP{}, fmt.Errorf("reqest ip \"%v\" is illeage", reqAddr)
			}

			if !appnetIPMap.SetBit(uint64(reqIndex), 1) {
				return net.IP{}, fmt.Errorf("IPbit map used fail")
			}

			pool.IPs[reqIP.String()] = reqIP
			return reqIP, nil
		}
	} else {
		//

		ipnet := pool.Subnet
		start := IPtoNum(pool.LowIp)
		end := IPtoNum(pool.MaxIp)
		/*
			start = uint32(index)
			offset := index - uint64(IPtoNum(ipnet.IP))
			logHandler.Debug("index:%v", index)
			GatewayIP = ipAdd(ipnet.IP, uint32(offset))
		*/

		//找到一个可用的IP地址
		var newip net.IP
		for {
			index, err := appnetIPMap.FindLowestUnsetBit(uint64(start), uint64(end))
			if err != nil {
				return net.IP{}, err
			}
			start = uint32(index)
			offset := index - uint64(IPtoNum(ipnet.IP))
			newip = ipAdd(ipnet.IP, uint32(offset))

			logHandler.Debug("check  ip %v is valid", newip.String())
			if !isvalidIP(newip) {
				//尝试对无效ip写1,避免以后被使用
				if !appnetIPMap.SetBit(index, 1) {
					//出错就忽视
					logHandler.Warn("try to set [%v] in ipbitmap fail", newip)
				}
				logHandler.Debug("get an invalid ip , skip, try next")

			} else { //设置指定ip为已使用
				if !appnetIPMap.SetBit(index, 1) {
					//出错就忽视
					logHandler.Error("try to set [%v] in ipbitmap fail", newip)
					return net.IP{}, errors.New("save ip address fail")
				}
				break

			}
		}
		logHandler.Debug("newIP:%v", newip.String())

		//记录ip和转换后的数值
		pool.IPs[newip.String()] = newip

		return newip, nil
	}
}

//net.IP返回供poolManager进行mac映射删除
func (pool *NetPool) ReleaseAddress(strAddr string) (net.IP, error) {
	logHandler.Debug("parsing ip addr [%v]", strAddr)
	ip := net.ParseIP(strAddr)
	if ip == nil {
		return net.IP{}, fmt.Errorf("can't parse ipaddr [%v]", strAddr)
	}

	index := IPtoNum(ip)

	if !appnetIPMap.SetBit(uint64(index), 0) {
		logHandler.Error("clean ip bit fail")
		return net.IP{}, errors.New("clean ip bit fail")
	}

	return ip, nil
}

func ParseCIDR(pool string) (net.IPNet, error) {
	ip, ipnet, err := net.ParseCIDR(pool)
	if err != nil {
		return net.IPNet{}, err
	}
	logHandler.Debug("ip:%v,ipnet:%v", ip.String(), ipnet.String())
	return *ipnet, nil

}

func (n *NetPools) ReleasePool(poolID string) error {
	if len(poolID) == 0 {
		return fmt.Errorf("invalid poolid")
	}

	pool, exists := n.Pools[poolID]
	if !exists {
		return fmt.Errorf("pool(%v) doesn't exists", poolID)
	}

	pool.put()

	if pool.Reference == 0 {
		for _, v := range pool.IPs {
			//1.清除mac地址和ip地址的映射
			for i, j := range n.MacMapping {
				if v.String() == j.String() {
					delete(n.MacMapping, i)
					break
				}
			}
			//2.清除ip位图
			index := IPtoNum(v)
			//更合理的处理
			appnetIPMap.SetBit(uint64(index), 0)
		}
		delete(n.Pools, poolID)
	}

	return nil
}

func ipSetIpMap(ip net.IP) {
	index := IPtoNum(ip)
	appnetIPMap.SetBit(uint64(index), 1)
}

//ipmap位图太大，无法保存在etcd中，需要在同步时设置
func syncIpMap() {
	for _, v := range PoolManager.Pools {
		for _, j := range v.IPs {
			ipSetIpMap(j)
		}
	}
}

func InitNetPools() *NetPools {
	var pools NetPools
	pools.Pools = make(map[string]NetPool)
	pools.MacMapping = make(map[string]net.IP)
	pools.rwlock = new(sync.RWMutex)

	return &pools

}

func init() {
}
