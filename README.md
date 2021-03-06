# IPAM驱动
---
## 需求
  服务主机（假设ip为`192.168.4.11`）
  测试主机一台
## 使用方法:
1.在本机上启动一个etcd服务器，使用2379的端口
`etcd --listen-client-urls http://127.0.0.1:2379 --listen-peer-urls http://127.0.0.1:2380 --advertise-client-urls http://127.0.0.1:2379
`

2.启动ipam服务器`sudo ./appipam`

3.在测试主机上`/etc/docker/plugins/appnet.spec`文件中，添加
`tcp://192.168.4.11:9527`

4.创建macvlan网络
`docker network create -d macvlan --ipam-driver=appnet --subnet=192.168.15.2/24 --gateway=192.168.15.1 -o parent=ens33 -o macvlan_mode=bridge ens33_1
`


## 编译方法
1.进入appipam代码目录
执行`docker build -f Dockerfile.build -t appipam-build .` 生成appipam-build镜像

2.返回到appipam父目录
执行`docker run -rm -v `pwd`:/src/appipam  -v `pwd`/dist:/src/dist appipam-build /src/scripts/build` 生成代码目录

3.编译后在程序文件在appipam父目录下dist目录中

