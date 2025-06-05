package mirageecs

import (
	"github.com/samber/lo"
)

type Recover struct {
	Enable             bool       `yaml:"enable"`
	Parameter          *Parameter `yaml:"parameter"`
	HookStoppedReasons []string   `yaml:"hook_stopped_reasons"`
}

func (r *Recover) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type _Recover Recover
	v := _Recover{
		Enable: false,
		Parameter: &Parameter{
			Name:     "Relaunch",
			Env:      "RELAUNCH",
			Default:  "on",
			Required: false,
			Internal: true,
		},
		HookStoppedReasons: []string{
			"ECS is performing maintenance on the underlying infrastructure hosting the task",
		},
	}
	if err := unmarshal(&v); err != nil {
		return err
	}
	*r = Recover(v)
	return nil
}

func (r *Recover) IsEnable() bool {
	return r != nil && r.Enable
}

func (r *Recover) AddOrSkipParameter(ps Parameters) Parameters {
	if r.IsEnable() && !lo.ContainsBy(ps, func(v *Parameter) bool {
		return v.Name == r.Parameter.Name
	}) {
		ps = append(ps, r.Parameter)
	}
	return ps
}

func (r *Recover) RecoverTaskParameter(param TaskParameter) TaskParameter {
	if !r.IsEnable() {
		return param
	}
	ret := make(TaskParameter, len(param)+1)
	for k, v := range param {
		ret[k] = v
	}
	ret[r.Parameter.Name] = r.Parameter.Default
	return ret
}
