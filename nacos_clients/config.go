package nacos_clients

import (
	"sync"

	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
)

type configClients struct {
	sync.RWMutex
	mapper map[string]config_client.IConfigClient
}

func newConfigClients() *configClients {
	return &configClients{
		mapper: map[string]config_client.IConfigClient{},
	}
}

func (cc *configClients) GetClient(serverName, nameSpace string) (client config_client.IConfigClient, ok bool) {
	key := clientMapKey(serverName, nameSpace)
	client, ok = cc.mapper[key]
	return
}

func (cc *configClients) SetClient(serverName, nameSpace string, client config_client.IConfigClient) {
	cc.Lock()
	defer cc.Unlock()
	key := clientMapKey(serverName, nameSpace)
	cc.mapper[key] = client
}
