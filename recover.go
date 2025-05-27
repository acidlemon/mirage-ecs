package mirageecs

import "fmt"

type Recover struct {
	ExcludeParameter map[string]struct{} `json:"exclude_parameters" yaml:"exclude_parameters"`
	FixedParameter   map[string]string   `json:"fixed_parameters" yaml:"fixed_parameters"`

	Parameter Parameters `json:"-" yaml:"-"`
}

func (p *Recover) Validate(ps Parameters) error {
	validParameterMap := make(map[string]struct{}, len(ps))
	for _, p := range ps {
		k := p.Name
		if _, ok := validParameterMap[k]; !ok {
			validParameterMap[k] = struct{}{}
		}
	}

	for k := range p.ExcludeParameter {
		if _, ok := validParameterMap[k]; !ok {
			return fmt.Errorf("exclude_parameters include invalid parameter: %s", k)
		}
	}
	for k := range p.FixedParameter {
		if _, ok := validParameterMap[k]; !ok {
			return fmt.Errorf("fixed_parameters include invalid parameter: %s", k)
		}
	}
	for k := range p.ExcludeParameter {
		if _, ok := p.FixedParameter[k]; ok {
			return fmt.Errorf("parameters conflicted: %s", k)
		}
	}

	p.Parameter = ps

	return nil
}
