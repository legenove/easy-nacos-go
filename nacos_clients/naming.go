package nacos_clients

import (
	"sync"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
)

type namingClients struct {
	sync.RWMutex
	mapper map[string]naming_client.INamingClient
}

func newNameClients() *namingClients {
	return &namingClients{
		mapper: map[string]naming_client.INamingClient{},
	}
}

func (nc *namingClients) GetClient(serverName, nameSpace string) (client naming_client.INamingClient, ok bool) {
	nc.RLock()
	defer nc.RUnlock()
	key := clientMapKey(serverName, nameSpace)
	client, ok = nc.mapper[key]
	return
}

func (nc *namingClients) SetClient(serverName, nameSpace string, client naming_client.INamingClient) {
	nc.Lock()
	defer nc.Unlock()
	key := clientMapKey(serverName, nameSpace)
	nc.mapper[key] = client
}
