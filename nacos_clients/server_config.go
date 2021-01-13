package nacos_clients

import (
	"sync"

	"github.com/nacos-group/nacos-sdk-go/common/constant"
)

var ServerConfigs *serverConfigs

func init() {
	ServerConfigs = newServerConfig()
}

type serverConfigs struct {
	sync.RWMutex
	configs map[string][]constant.ServerConfig
}

func newServerConfig() *serverConfigs {
	return &serverConfigs{
		configs: map[string][]constant.ServerConfig{},
	}
}

func SetServerConfigs(nacosServerName string, configs ...constant.ServerConfig) {
	ServerConfigs.SetConfigs(nacosServerName, configs...)
}
func (sc *serverConfigs) SetConfigs(nacosServerName string, configs ...constant.ServerConfig) {
	if len(configs) == 0 {
		return
	}
	sc.Lock()
	defer sc.Unlock()
	var v []constant.ServerConfig
	var ok bool
	if v, ok = sc.configs[nacosServerName]; !ok {
		v = make([]constant.ServerConfig, 0, len(configs))
	}
	sc.configs[nacosServerName] = append(v, configs...)
}

func GetServerConfigs(nacosServerName string) (configs []constant.ServerConfig, ok bool) {
	return ServerConfigs.GetConfigs(nacosServerName)
}
func (sc *serverConfigs) GetConfigs(serverName string) (configs []constant.ServerConfig, ok bool) {
	sc.RLock()
	defer sc.RUnlock()
	configs, ok = sc.configs[serverName]
	if len(configs) == 0 {
		ok = false
	}
	return
}
