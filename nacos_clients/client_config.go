package nacos_clients

import (
	"sync"

	"github.com/nacos-group/nacos-sdk-go/common/constant"
)

var ClientConfigs *clientConfig

func init() {
	ClientConfigs = newClientConfig()
}

type clientConfig struct {
	sync.RWMutex
	configs map[string]constant.ClientConfig
}

func newClientConfig() *clientConfig {
	return &clientConfig{
		configs: map[string]constant.ClientConfig{},
	}
}

func SetClientConfigs(nameSpace string, config constant.ClientConfig) {
	ClientConfigs.SetConfigs(nameSpace, config)
}
func (cc *clientConfig) SetConfigs(nameSpace string, config constant.ClientConfig) {
	cc.Lock()
	defer cc.Unlock()
	cc.configs[nameSpace] = config
}

func GetClientConfigs(nameSpace string) (config constant.ClientConfig, ok bool) {
	return ClientConfigs.GetConfigs(nameSpace)
}
func (cc *clientConfig) GetConfigs(nameSpace string) (config constant.ClientConfig, ok bool) {
	cc.RLock()
	defer cc.RUnlock()
	config, ok = cc.configs[nameSpace]
	return
}
