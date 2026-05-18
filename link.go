package mirageecs

import "path"

func (l *Link) isExcluded(containerName string) bool {
	if l.HostedZoneID == "" {
		return true
	}
	for _, excluded := range l.ExcludeContainers {
		if m, _ := path.Match(excluded, containerName); m {
			return true
		}
	}
	return false
}
