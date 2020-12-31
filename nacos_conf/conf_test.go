package nacos_conf

import (
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"sync"
	"testing"
)

func TestNacosConfig_AllKeys(t *testing.T) {

}

func TestNewConfManage(t *testing.T) {
	sc := []constant.ServerConfig{
		{
			IpAddr: "127.0.0.1",
			Port:   8848,
		},
	}

	cc := constant.ClientConfig{
		NamespaceId:         "galaxy-could", //namespace id
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "./log/nacos/log",
		CacheDir:            "./log/nacos/cache",
		RotateTime:          "1h",
		MaxAge:              3,
		LogLevel:            "debug",
	}

	client, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil  {
		panic(err)
	}
	cm := NewConfManage("galaxy-could", "dev", "web_demo_", client)
	_, _ = cm.Instance("app.yaml", "yaml", nil)
	wg := sync.WaitGroup{}

	_, _ = cm.Instance("app1.yaml", "yaml", nil)
	wg.Add(1)
	wg.Wait()
}
