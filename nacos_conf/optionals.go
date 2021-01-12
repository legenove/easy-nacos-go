package nacos_conf

import "github.com/legenove/easyconfig/ifacer"

// 私有的optional

func OptConfGroup(group string) ifacer.OptionFunc {
	return func(iv ifacer.Configer) {
		v, ok := iv.(*NacosConfig)
		if ok {
			v.group = group
		}
	}
}

func OptConfDataIdPrefix(pre string) ifacer.OptionFunc {
	return func(iv ifacer.Configer) {
		v, ok := iv.(*NacosConfig)
		if ok {
			v.dataIdPrefix = pre
		}
	}
}
