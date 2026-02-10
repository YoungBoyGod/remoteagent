package config

import "os"

type Config struct {
	LocalAddr string
}

func Load() Config {
	addr := os.Getenv("AGENT_LOCAL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:40002"
	}
	return Config{LocalAddr: addr}
}
