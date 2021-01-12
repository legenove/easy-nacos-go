package nacos_conf

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/legenove/easyconfig/config_tools"
	"github.com/legenove/easyconfig/ifacer"
	"github.com/legenove/easyconfig/parsers"
	"github.com/legenove/utils"
	"github.com/magiconair/properties"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/spf13/cast"
)

// TODO 定时查询

type NacosConfManage struct {
	sync.RWMutex
	NameSpace    string
	Group        string
	DataIdPrefix string
	ConfigClient config_client.IConfigClient
	mapper       map[string]*NacosConfig
}

func NewConfManage(nameSpace, group, dataIdPrefix string, configClient config_client.IConfigClient) *NacosConfManage {
	return &NacosConfManage{
		NameSpace:    nameSpace,
		Group:        group,
		DataIdPrefix: dataIdPrefix,
		ConfigClient: configClient,
		mapper:       map[string]*NacosConfig{},
	}
}

func (n *NacosConfManage) OnchangeFunc(namespace, group, dataId, data string) {
	nv, vok := n.mapper[mapperCacheKey(group, dataId)]
	if !vok {
		return
	}
	if len(data) > 0 {
		err := nv.setChange(data)
		if nv.OnChangeFunc != nil && err == nil {
			nv.OnChangeFunc(nv)
		}
	} else {
		nv.setRemove()
		if nv.OnRemoveFunc != nil {
			nv.OnRemoveFunc(nv)
		}
	}
}

func (n *NacosConfManage) Instance(dataId, _type string, val interface{}, opts ...ifacer.OptionFunc) (ifacer.Configer, error) {
	n.Lock()
	defer n.Unlock()
	var v = NewNacosConfig(n, dataId)
	v.SetConfType(_type)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(v)
	}
	_v, ok := n.mapper[mapperCacheKey(v.group, v.GetFullName())]
	if ok {
		return _v, nil
	}
	go v.GetConf()
	err := n.ConfigClient.ListenConfig(vo.ConfigParam{
		DataId:   v.GetFullName(),
		Group:    v.group,
		OnChange: n.OnchangeFunc,
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	v.Val = val
	n.mapper[mapperCacheKey(v.group, v.GetFullName())] = v
	return v, nil
}

func mapperCacheKey(group, fullName string) string {
	return utils.ConcatenateStrings(fullName, "@", group)
}

type NacosConfig struct {
	BaseConf      *NacosConfManage       // 全局配置
	ConfLock      sync.Mutex             // 设置配置的锁
	ValLock       sync.Mutex             // 格式化值的锁
	LoadLock      sync.Mutex             // 读配置的锁
	dataIdPrefix  string                 // data prefix
	dataId        string                 // dataId
	group         string                 // 组名
	Type          string                 // 文件类型
	HasVal        bool                   // 是否有值
	Val           interface{}            // 配置的格式化的值
	md5Ver        string                 // 版本
	onChange      chan struct{}          // 变更通知
	onRemove      chan struct{}          // 变更通知
	OnChangeFunc  ifacer.ChangeFunc      // 改变 func
	OnRemoveFunc  ifacer.ChangeFunc      // 删除 func
	readingConfig bool                   // 是否正在读配置
	config        map[string]interface{} // 缓存的格式化配置
	keyDelim      string                 // key分隔符
	Error         error

	// Store read properties on the object so that we can write back in order with comments.
	// This will only be used if the configuration read is a properties file.
	properties *properties.Properties
}

func NewNacosConfig(manager *NacosConfManage, dataId string) *NacosConfig {
	return &NacosConfig{
		BaseConf:     manager,
		onChange:     make(chan struct{}, 8),
		onRemove:     make(chan struct{}, 8),
		dataId:       dataId,
		dataIdPrefix: manager.DataIdPrefix,
		group:        manager.Group,
		keyDelim:     ".",
		OnChangeFunc: DefaultOnChangeFunc,
		OnRemoveFunc: DefaultOnRemoveFunc,
	}
}

func (n *NacosConfig) GetProperties() *properties.Properties {
	return n.properties
}

func (n *NacosConfig) SetProperties(p *properties.Properties) {
	n.properties = p
}

func (n *NacosConfig) GetName() string {
	return n.dataId
}

func (n *NacosConfig) GetFullName() string {
	if len(n.dataIdPrefix) != 0 {
		return utils.ConcatenateStrings(n.dataIdPrefix, n.dataId)
	}
	return n.dataId
}

func (n *NacosConfig) GetConfType() string {
	return n.Type
}

func (n *NacosConfig) SetConfType(t string) {
	n.Type = t
}

func (n *NacosConfig) readConfNoLock() error {
	client := n.BaseConf.ConfigClient
	content, err := client.GetConfig(vo.ConfigParam{
		DataId: n.GetFullName(),
		Group:  n.group,
	})
	if err != nil {
		return err
	}
	if len(content) > 0 {
		err = n.setChange(content)
	} else {
		n.setRemove()
	}
	return err
}

func (n *NacosConfig) GetConf() error {
	if len(n.keyDelim) == 0 {
		n.keyDelim = "."
	}
	n.LoadLock.Lock()
	defer n.LoadLock.Unlock()
	err := n.readConfNoLock()
	if err == nil {
		if n.hasConfig() {
			n.OnChangeFunc(n)
		}
	}
	return err
}

func (n *NacosConfig) hasConfig() bool {
	n.ConfLock.Lock()
	defer n.ConfLock.Unlock()
	return !(n.config == nil || len(n.config) == 0)
}

func (n *NacosConfig) setConfig(md5 string, config map[string]interface{}) {
	n.ConfLock.Lock()
	defer n.ConfLock.Unlock()
	n.config = config
	n.md5Ver = md5
}

func (n *NacosConfig) setChange(content string) error {
	md5Ver := utils.GetMD5Hash(content)
	if n.sameVersion(md5Ver) {
		return nil
	}
	config, err := n.unmarshalContent(content)
	if err == nil {
		n.setConfig(md5Ver, config)
	}
	fmt.Println(111, config)
	return err
}

func (n *NacosConfig) setRemove() {
	n.setConfig("", map[string]interface{}{})
}

func (n *NacosConfig) sameVersion(md5 string) bool {
	n.ConfLock.Lock()
	defer n.ConfLock.Unlock()
	if len(n.md5Ver) == 0 {
		return false
	}
	if n.md5Ver == md5 {
		return true
	}
	return false
}

func (n *NacosConfig) AllKeys() []string {
	m := map[string]bool{}
	// add all paths, by order of descending priority to ensure correct shadowing
	m = n.flattenAndMergeMap(m, n.config, "")
	// convert set of paths to list
	a := make([]string, 0, len(m))
	for x := range m {
		a = append(a, x)
	}
	return a
}

func (n *NacosConfig) AllSettings() map[string]interface{} {
	m := map[string]interface{}{}
	// start from the list of keys, and construct the map one value at a time
	for _, k := range n.AllKeys() {
		value := n.get(k)
		if value == nil {
			// should not happen, since AllKeys() returns only keys holding a value,
			// check just in case anything changes
			continue
		}
		path := strings.Split(k, n.keyDelim)
		lastKey := strings.ToLower(path[len(path)-1])
		deepestMap := config_tools.DeepSearch(m, path[0:len(path)-1])
		// set innermost value
		deepestMap[lastKey] = value
	}
	return m
}

func (n *NacosConfig) Unmarshal(rawVal interface{}) error {
	return decode(n.AllSettings(), defaultDecoderConfig(rawVal))
}

func (n *NacosConfig) unmarshalContent(content string) (map[string]interface{}, error) {
	config := make(map[string]interface{})
	p, ok := parsers.SupportedParsers[strings.ToLower(n.GetConfType())]
	if ok {
		if n.config == nil {
			n.config = map[string]interface{}{}
		}
		err := p.UnmarshalReader(n, strings.NewReader(content), config)
		if err != nil {
			return nil, config_tools.ConfigParseError{Err: err}
		}
	}
	config_tools.InsensitiviseMap(config)
	return config, nil
}

func (n *NacosConfig) GetValue() interface{} {
	if !n.hasConfig() {
		n.LoadLock.Lock()
		if !n.hasConfig() {
			err := n.readConfNoLock()
			n.LoadLock.Unlock()
			if err != nil {
				return nil
			}
		}
		n.LoadLock.Unlock()
	}
	if n.Val != nil && !n.HasVal && n.OnChangeFunc != nil {
		n.LoadLock.Lock()
		if n.Val != nil && !n.HasVal {
			n.LoadLock.Unlock()
			n.OnChangeFunc(n)
		}
		n.LoadLock.Unlock()
	}
	return n.Val
}

func (n *NacosConfig) Get(key string) (interface{}, error) {
	return n.get(key), nil
}

func (n *NacosConfig) GetString(key string) (string, error) {
	return cast.ToStringE(n.get(key))
}

func (n *NacosConfig) GetBool(key string) (bool, error) {
	return cast.ToBoolE(n.get(key))
}

func (n *NacosConfig) GetInt(key string) (int, error) {
	return cast.ToIntE(n.get(key))
}

func (n *NacosConfig) GetInt32(key string) (int32, error) {
	return cast.ToInt32E(n.get(key))
}

func (n *NacosConfig) GetInt64(key string) (int64, error) {
	return cast.ToInt64E(n.get(key))
}

func (n *NacosConfig) GetFloat64(key string) (float64, error) {
	return cast.ToFloat64E(n.get(key))
}

func (n *NacosConfig) GetTime(key string) (time.Time, error) {
	return cast.ToTimeE(n.get(key))
}

func (n *NacosConfig) GetDuration(key string) (time.Duration, error) {
	return cast.ToDurationE(n.get(key))
}

func (n *NacosConfig) GetStringSlice(key string) ([]string, error) {
	return cast.ToStringSliceE(n.get(key))
}

func (n *NacosConfig) GetStringMap(key string) (map[string]interface{}, error) {
	return cast.ToStringMapE(n.get(key))
}

func (n *NacosConfig) GetStringMapString(key string) (map[string]string, error) {
	return cast.ToStringMapStringE(n.get(key))
}

func (n *NacosConfig) GetStringMapStringSlice(key string) (map[string][]string, error) {
	return cast.ToStringMapStringSliceE(n.get(key))
}

func (n *NacosConfig) GetSizeInBytes(key string) (uint, error) {
	return cast.ToUint(n.get(key)), nil
}

func (n *NacosConfig) UnmarshalKey(key string, rawVal interface{}) error {
	return decode(n.get(key), defaultDecoderConfig(rawVal))
}

func (n *NacosConfig) SetOnChangeFunc(onChangeFunc ifacer.ChangeFunc) {
	n.OnChangeFunc = onChangeFunc
}

func (n *NacosConfig) SetOnRemoveFunc(onRemoveFunc ifacer.ChangeFunc) {
	n.OnRemoveFunc = onRemoveFunc
}

func (n *NacosConfig) get(key string) interface{} {
	lcaseKey := strings.ToLower(key)
	val := n.find(lcaseKey, true)
	if val == nil {
		return nil
	}
	return val
}

func (n *NacosConfig) find(lcaseKey string, flagDefault bool) interface{} {
	if n.config == nil {
		return nil
	}
	var (
		val  interface{}
		path = strings.Split(lcaseKey, n.keyDelim)
	)

	// Config file next
	val = n.searchMapWithPathPrefixes(n.config, path)
	if val != nil {
		return val
	}
	return nil
}

func (n *NacosConfig) OnChangeChan() <-chan struct{} {
	return n.onChange
}

func (n *NacosConfig) OnRemoveChan() <-chan struct{} {
	return n.onRemove
}

// searchMapWithPathPrefixes recursively searches for a value for path in source map.
//
// While searchMap() considers each path element as a single map key, this
// function searches for, and prioritizes, merged path elements.
// e.g., if in the source, "foo" is defined with a sub-key "bar", and "foo.bar"
// is also defined, this latter value is returned for path ["foo", "bar"].
//
// This should be useful only at config level (other maps may not contain dots
// in their keys).
//
// Note: This assumes that the path entries and map keys are lower cased.
func (n *NacosConfig) searchMapWithPathPrefixes(source map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return source
	}
	// search for path prefixes, starting from the longest one
	for i := len(path); i > 0; i-- {
		prefixKey := strings.ToLower(strings.Join(path[0:i], n.keyDelim))

		next, ok := source[prefixKey]
		if ok {
			// Fast path
			if i == len(path) {
				return next
			}

			// Nested case
			var val interface{}
			switch next.(type) {
			case map[interface{}]interface{}:
				val = n.searchMapWithPathPrefixes(cast.ToStringMap(next), path[i:])
			case map[string]interface{}:
				// Type assertion is safe here since it is only reached
				// if the type of `next` is the same as the type being asserted
				val = n.searchMapWithPathPrefixes(next.(map[string]interface{}), path[i:])
			default:
				// got a value but nested key expected, do nothing and look for next prefix
			}
			if val != nil {
				return val
			}
		}
	}
	// not found
	return nil
}

// flattenAndMergeMap recursively flattens the given map into a map[string]bool
// of key paths (used as a set, easier to manipulate than a []string):
// - each path is merged into a single key string, delimited with v.keyDelim
// - if a path is shadowed by an earlier value in the initial shadow map,
//   it is skipped.
// The resulting set of paths is merged to the given shadow set at the same time.
func (n *NacosConfig) flattenAndMergeMap(shadow map[string]bool, m map[string]interface{}, prefix string) map[string]bool {
	if shadow != nil && prefix != "" && shadow[prefix] {
		// prefix is shadowed => nothing more to flatten
		return shadow
	}
	if shadow == nil {
		shadow = make(map[string]bool)
	}

	var m2 map[string]interface{}
	if prefix != "" {
		prefix += n.keyDelim
	}
	for k, val := range m {
		fullKey := prefix + k
		switch val.(type) {
		case map[string]interface{}:
			m2 = val.(map[string]interface{})
		case map[interface{}]interface{}:
			m2 = cast.ToStringMap(val)
		default:
			// immediate value
			shadow[strings.ToLower(fullKey)] = true
			continue
		}
		// recursively merge to shadow map
		shadow = n.flattenAndMergeMap(shadow, m2, fullKey)
	}
	return shadow
}

func DefaultOnChangeFunc(iv ifacer.Configer) {
	v, ok := iv.(*NacosConfig)
	if !ok {
		return
	}
	if v.Val != nil && v.hasConfig() {
		v.ValLock.Lock()
		defer v.ValLock.Unlock()
		err := v.Unmarshal(&v.Val)
		if err != nil {
			return
		}
		v.HasVal = true
	}
	go func() {
		// 防止无人使用 onchange channel
		if len(v.onChange) == 0 {
			select {
			case v.onChange <- struct{}{}:
			case <-time.After(time.Millisecond):
			}
		}
	}()
}

func DefaultOnRemoveFunc(iv ifacer.Configer) {
	v, ok := iv.(*NacosConfig)
	if !ok {
		return
	}
	if v.hasConfig() {
		v.ConfLock.Lock()
		defer v.ConfLock.Unlock()
		if !(v.config == nil || len(v.config) == 0) {
			return
		}
		v.config = map[string]interface{}{}
	}
}
