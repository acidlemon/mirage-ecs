package mirageecs

import "path"

func (l *Link) shouldRegisterRecord(containerName string) bool {
	if l.HostedZoneID == "" {
		return false
	}
	for _, excluded := range l.ExcludeContainers {
		if m, _ := path.Match(excluded, containerName); m {
			return false
		}
	}
	return true
}
