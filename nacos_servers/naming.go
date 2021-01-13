package nacos_servers

import (
	"github.com/legenove/easy-nacos-go/nacos_clients"
	"github.com/legenove/utils"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
)

type ServerManageParam struct {
	NameSpace       string
	Group           string
	Ip              string
	Port            uint64
	ServerName      string
	Weight          float64
	Metadata        map[string]string
	ClusterName     string
	NacosServerName string
	AllClusters     []string
}

type UpdateServerManageParam struct {
	NacosServerName string
	Weight          float64
	Metadata        map[string]string
}

type NacosServerManage struct {
	sync.RWMutex
	NacosServerName  string // nacos默认集群名字
	NameSpace        string
	Group            string
	Ip               string
	Port             uint64
	ServerName       string
	Weight           float64
	Metadata         map[string]string
	ClusterName      string
	NamingClient     map[string]naming_client.INamingClient
	SubscribeServers map[string]*NacosServer
}

func NewNacosServerManage(client naming_client.INamingClient, param ServerManageParam) *NacosServerManage {
	nsm := &NacosServerManage{
		NacosServerName: param.NacosServerName,
		NameSpace:       param.NameSpace,
		Group:           param.Group,
		Ip:              param.Ip,
		Port:            param.Port,
		ServerName:      param.ServerName,
		Weight:          param.Weight,
		Metadata:        param.Metadata,
		ClusterName:     param.ClusterName,
		NamingClient:    map[string]naming_client.INamingClient{param.NameSpace: client},
	}
	return nsm
}

func (n *NacosServerManage) UpdateServerInfo(param UpdateServerManageParam) {
	n.DeregisterServer()
	if param.Weight > 0 {
		n.Weight = param.Weight
	}
	if len(param.Metadata) > 0 {
		n.Metadata = param.Metadata
	}
	if len(param.NacosServerName) > 0 {
		n.NacosServerName = param.NacosServerName
	}
	n.RegisterServer()
}

func (n *NacosServerManage) subsribeKey(nameSpace, group, serviceName string) string {
	return utils.ConcatenateStrings(nameSpace, "@", group, "@", serviceName)
}

func (n *NacosServerManage) SubscribeServer(nameSpace, group, serviceName string,
	cluster []string, client naming_client.INamingClient, onChange func(services []model.SubscribeService, err error)) *NacosServer {
	key := n.subsribeKey(nameSpace, group, serviceName)
	n.Lock()
	defer n.Unlock()
	ns, ok := n.SubscribeServers[key]
	if !ok {
		ns := NewNacosServer(client, cluster, serviceName, group, onChange)
		n.SubscribeServers[key] = ns
	}
	return ns
}

func (n *NacosServerManage) GetSubscribeServer(nameSpace, group, serviceName string) (*NacosServer, bool) {
	key := n.subsribeKey(nameSpace, group, serviceName)
	n.RLock()
	defer n.RUnlock()
	ns, ok := n.SubscribeServers[key]
	return ns, ok
}

func (n *NacosServerManage) RegisterNamingClient(nameSpace string, client naming_client.INamingClient) {
	n.Lock()
	defer n.Unlock()
	n.NamingClient[nameSpace] = client
}

func (n *NacosServerManage) GetNamingClient(nameSpace string) (naming_client.INamingClient, bool) {
	n.RLock()
	defer n.RUnlock()
	client, ok := n.NamingClient[nameSpace]
	if !ok && len(n.NacosServerName) > 0 {
		client, ok = nacos_clients.GetNamingClient(n.NacosServerName, nameSpace)
	}
	return client, ok
}

func (n *NacosServerManage) RegisterServer() (bool, error) {
	client, _ := n.GetNamingClient(n.NameSpace)
	return client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          n.Ip,
		Port:        n.Port,
		ServiceName: n.ServerName,
		Weight:      n.Weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    n.Metadata,
		ClusterName: n.ClusterName, // 默认值DEFAULT
		GroupName:   n.Group,       // 默认值DEFAULT_GROUP
	})
}

func (n *NacosServerManage) DeregisterServer() (bool, error) {
	client, _ := n.GetNamingClient(n.NameSpace)
	return client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          n.Ip,
		Port:        n.Port,
		ServiceName: n.ServerName,
		GroupName:   n.Group, // 默认值DEFAULT_GROUP
		Ephemeral:   true,
	})
}

type NacosServer struct {
	client      naming_client.INamingClient
	Cluster     []string
	ServiceName string
	GroupName   string
	OnChange    func(services []model.SubscribeService, err error)
}

func NewNacosServer(client naming_client.INamingClient,
	cluster []string,
	serviceName string,
	group string, onChange func(services []model.SubscribeService, err error)) *NacosServer {
	ns := &NacosServer{
		client:      client,
		Cluster:     cluster,
		ServiceName: serviceName,
		GroupName:   group,
		OnChange:    onChange,
	}
	ns.ListenServer()
	return ns
}

func (n *NacosServer) GetServers() ([]model.SubscribeService, error) {
	s1, err := n.client.GetService(vo.GetServiceParam{
		Clusters:    n.Cluster,
		ServiceName: n.ServiceName,
		GroupName:   n.GroupName,
	})
	if err != nil {
		return nil, err
	}
	res := make([]model.SubscribeService, 0, len(s1.Hosts))
	for _, h := range s1.Hosts {
		if h.Valid && h.Weight > 0 {
			res = append(res, model.SubscribeService{
				ClusterName: h.ClusterName,
				Enable:      h.Enable,
				InstanceId:  h.InstanceId,
				Ip:          h.Ip,
				Metadata:    h.Metadata,
				Port:        h.Port,
				ServiceName: h.ServiceName,
				Valid:       h.Valid,
				Weight:      h.Weight,
			})
		}
	}
	n.ListenServer()
	return res, nil
}

func (n *NacosServer) UnListenServer() {
	_ = n.client.Unsubscribe(&vo.SubscribeParam{
		Clusters:          n.Cluster,
		ServiceName:       n.ServiceName,
		GroupName:         n.GroupName,
		SubscribeCallback: n.OnChange,
	})
}

func (n *NacosServer) UpdateCluster(cluster []string) {
	n.UnListenServer()
	n.Cluster = cluster
	n.ListenServer()
}

func (n *NacosServer) UpdateOnChange(onChange func(services []model.SubscribeService, err error)) {
	n.UnListenServer()
	n.OnChange = onChange
	n.ListenServer()
}

func (n *NacosServer) ListenServer() {
	_ = n.client.Subscribe(&vo.SubscribeParam{
		Clusters:          n.Cluster,
		ServiceName:       n.ServiceName,
		GroupName:         n.GroupName,
		SubscribeCallback: n.OnChange,
	})
}
