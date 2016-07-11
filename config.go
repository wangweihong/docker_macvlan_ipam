package main

import "flag"

type DriverConfig struct {
	PluginDir  string
	DriverName string
	BackendUrl string
}

type Config struct {
	DriverConfig
}

func LoadConfig() (config *Config) {
	config = new(Config)

	flag.StringVar(&config.PluginDir, "plugin-dir", "/run/docker/plugins", "Docker plugin directory where driver socket is created")
	flag.StringVar(&config.DriverName, "driver-name", "appnet", "Name of appnet IPAM driver")
	flag.StringVar(&config.BackendUrl, "backend-url", "127.0.0.1:2379", "backend store's url address")

	flag.Parse()
	return config
}
