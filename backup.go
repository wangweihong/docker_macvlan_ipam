package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/etcd"
)

var (
	watchKey      = "/"
	appnetDirNode = "/appnet"
	storeKey      = appnetDirNode + "/appipam/ipam"
	BackendClient Backend

	//	defaultEndpoint = "127.0.0.1:2379"
	ErrBackendKeyNotFound = store.ErrKeyNotFound
)

type backend struct {
	kv    store.Store
	alive bool
	//	defaultEndpoint string
}

func NewBackend(defaultEndpoint string) *backend {
	etcd.Register()
	st, err := libkv.NewStore(
		store.ETCD,
		[]string{defaultEndpoint}, &store.Config{
			ConnectionTimeout: 5 * time.Second,
		},
	)
	if err != nil {
		return nil

	}

	b := &backend{}
	b.kv = st

	//做一次健康检查
	/*
		if !b.healthCheckOnce() {
			logHandler.Debug("backend healthy check fail for %v", store.ErrNotReachable.Error())
			return nil
		}

	*/
	return b
}

func (b *backend) Alive() bool {
	return b.alive

}

//树的形状
//  /-
//   |- poo1--/- ip..
//   |- pool--/- ip..
func (b *backend) Save(data interface{}) error {
	if !b.Alive() {
		return fmt.Errorf("can not connect to backend")
	}

	byteContent, err := json.Marshal(data)
	if err != nil {
		return err
	}

	//	err = b.kv.Put(storeKey, byteContent, &store.WriteOptions{IsDir: true})
	err = b.kv.Put(storeKey, byteContent, nil)
	if err != nil {
		return err
	}

	return nil
}

func (b *backend) Get(data interface{}) error {
	if !b.Alive() {
		return fmt.Errorf("can not connect to backend")
	}

	kvpair, err := b.kv.Get(storeKey)
	if err != nil {
		return err
	}

	err = json.Unmarshal(kvpair.Value, data)
	if err != nil {
		return err
	}

	return nil

	//	kv.

}

func (b *backend) Remove(data interface{}) error {
	if !b.Alive() {
		return fmt.Errorf("can not connect to backend")
	}

	err := b.kv.Delete(storeKey)
	if err != nil {
		return err
	}

	return nil
}

func (b *backend) healthCheckOnce() bool {
	_, err := b.kv.List(watchKey)
	//	if err != nil && err.Error() == store.ErrNotReachable.Error() {
	if err != nil {
		return false
	} else {
		return true
	}
}

func (b *backend) HealthCheck() {
	var count1 int
	var count2 int
	logHandler.Debug("start to health check...")
	for {
		if b.healthCheckOnce() {
			//do something, if true
			b.alive = true
			if count1 == 0 {
				logHandler.Debug("result:healthy")
				//将count2 统计请0,以便一旦断开健康检测失败后,能作出相应的打印
				if count2 != 0 {
					count2 = 0
				}
			}
			count1 += 1
			if count1 == 100 {
				count1 = 0
			}
		} else {
			b.alive = false
			if count2 == 0 {
				logHandler.Debug("result:unhealthy")
				if count1 != 0 {
					count1 = 0
				}
			}
			count2 += 1
			if count2 == 20 {
				count2 = 0
			}
		}

		//每隔一秒进行一次健康检查
		time.Sleep(1 * time.Second)
	}

}

type Backend interface {
	Alive() bool
	Save(interface{}) error
	Remove(interface{}) error
	Get(interface{}) error
	HealthCheck()
}

/*

type Test struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func main() {

	test := Test{A: 404, B: "not found"}
	err := BackendClient.Save(test)
	if err != nil {
		fmt.Printf("Err:%v\n", err.Error())
	}

	time.Sleep(100000 * time.Second)
}
*/

func init() {

}
