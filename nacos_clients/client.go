package nacos_clients

import (
	"errors"
	"sync"

	"github.com/legenove/utils"
	nacosClients "github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/common/logger"
)

var Clients *clients

func init() {
	Clients = newClient()
}

type clients struct {
	sync.RWMutex
	Namings *namingClients
	Configs *configClients
	logger  logger.Logger
}

func newClient() *clients {
	return &clients{
		Namings: newNameClients(),
		Configs: newConfigClients(),
	}
}

func NewNamingClient(serverName, nameSpace string) error {
	sc, cc, err := getServerAndClientConfig(serverName, nameSpace)
	if err != nil {
		return err
	}
	// 创建服务发现客户端
	namingClient, err := nacosClients.CreateNamingClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		return err
	}
	Clients.SetNamingClient(serverName, nameSpace, namingClient)
	return nil
}
func (c *clients) SetNamingClient(serverName, nameSpace string, client naming_client.INamingClient) {
	c.Namings.SetClient(serverName, nameSpace, client)
	// 每次新建，都会覆盖logger，所以这里重新覆盖一次
	c.recoverLogger()
}

func NewConfigClient(serverName, nameSpace string) error {
	sc, cc, err := getServerAndClientConfig(serverName, nameSpace)
	if err != nil {
		return err
	}
	// 创建配置中心客户端
	configClient, err := nacosClients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		return err
	}
	Clients.SetConfigClient(serverName, nameSpace, configClient)
	return nil

}
func (c *clients) SetConfigClient(serverName, nameSpace string, client config_client.IConfigClient) {
	c.Configs.SetClient(serverName, nameSpace, client)
	// 每次新建，都会覆盖logger，所以这里重新覆盖一次
	c.recoverLogger()
}
func GetNamingClient(serverName, nameSpace string) (client naming_client.INamingClient, ok bool) {
	return Clients.GetNamingClient(serverName, nameSpace)
}
func (c *clients) GetNamingClient(serverName, nameSpace string) (client naming_client.INamingClient, ok bool) {
	return c.Namings.GetClient(serverName, nameSpace)
}

func GetConfigClient(serverName, nameSpace string) (client config_client.IConfigClient, ok bool) {
	return Clients.GetConfigClient(serverName, nameSpace)
}
func (c *clients) GetConfigClient(serverName, nameSpace string) (client config_client.IConfigClient, ok bool) {
	return c.Configs.GetClient(serverName, nameSpace)
}

func SetClientLogger(l logger.Logger) {
	Clients.SetLogger(l)
	Clients.recoverLogger()
}
func (c *clients) SetLogger(l logger.Logger) {
	c.Lock()
	defer c.Unlock()
	c.logger = l
}

func (c *clients) GetLogger() logger.Logger {
	c.RLock()
	defer c.RUnlock()
	return c.logger
}

func (c *clients) recoverLogger() {
	l := c.GetLogger()
	if l != nil {
		logger.SetLogger(l)
	}
}

func clientMapKey(serverName, nameSpace string) string {
	return utils.ConcatenateStrings(serverName, nameSpace)
}

func getServerAndClientConfig(serverName, nameSpace string) (sc []constant.ServerConfig, cc constant.ClientConfig, err error) {
	var ok bool
	sc, ok = ServerConfigs.GetConfigs(serverName)
	if !ok {
		err = errors.New("nacos server config not found")
		return
	}
	cc, ok = ClientConfigs.GetConfigs(nameSpace)
	if !ok {
		err = errors.New("nacos client config not found")
		return
	}
	return
}
