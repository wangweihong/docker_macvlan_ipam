package main

import "flag"

type DriverConfig struct {
	PluginDir  string
	DriverName string
}

type Config struct {
	DriverConfig
}

func LoadConfig() (config *Config) {
	config = new(Config)

	flag.StringVar(&config.PluginDir, "plugin-dir", "/run/docker/plugins", "Docker plugin directory where driver socket is created")
	flag.StringVar(&config.DriverName, "driver-name", "appnet", "Name of appnet IPAM driver")

	flag.Parse()
	return config
}
