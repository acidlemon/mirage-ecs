package mirageecs

var (
	ValidateSubdomain = validateSubdomain
	NewHTTPTransport  = newHTTPTransport
	LinkIsExcluded    = (*Link).isExcluded
)

type Route53API = route53API

var NewRoute53WithClient = newRoute53
