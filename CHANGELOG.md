# Changelog

## [v2.3.0](https://github.com/acidlemon/mirage-ecs/compare/v2.2.5...v2.3.0) - 2026-11-13

- Update dependencies and require Go 1.26+ by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/120
- Replace rproxy with net/http/httputil.ReverseProxy by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/121
- Add an end-to-end test against AWS Emulator by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/122
- Skip Route53 DELETE for record sets that no longer exist by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/123
- Use distroless as the base of the container image by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/124
- Introduce tagpr to release by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/125

## [v2.2.5](https://github.com/acidlemon/mirage-ecs/compare/v2.2.4...v2.2.5) - 2026-07-27

- Bump golang.org/x/crypto from 0.45.0 to 0.52.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/102
- Bump golang.org/x/net from 0.54.0 to 0.55.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/101
- Rework terraform example as a disposable environment by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/105
- update aws-sdk-go-v2* to latest by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/104
- Require Go 1.25+ in CI workflows by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/114

## [v2.2.4](https://github.com/acidlemon/mirage-ecs/compare/v2.2.3...v2.2.4) - 2026-05-18

- Bump github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs from 1.22.1 to 1.65.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/95
- Feature/update aws sdk go v2 latest by @acidlemon in https://github.com/acidlemon/mirage-ecs/pull/100
- add exclude_containers to link config for filtering Route53 record registration by @shotaogasawara in https://github.com/acidlemon/mirage-ecs/pull/98
- Add status query parameter to GET /api/list by @shotaogasawara in https://github.com/acidlemon/mirage-ecs/pull/99

## [v2.2.3](https://github.com/acidlemon/mirage-ecs/compare/v2.2.2...v2.2.3) - 2025-12-17

- go version maintenance by @acidlemon in https://github.com/acidlemon/mirage-ecs/pull/94

## [v2.2.2](https://github.com/acidlemon/mirage-ecs/compare/v2.2.1...v2.2.2) - 2025-12-12

- fix callback url. by @nukokusa in https://github.com/acidlemon/mirage-ecs/pull/87
- upgrade echo to v4.13.3 to remove "github.com/golang-jwt/jwt v3.2.2" by @acidlemon in https://github.com/acidlemon/mirage-ecs/pull/89
- Bump golang.org/x/net from 0.36.0 to 0.38.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/88
- Bump golang.org/x/crypto from 0.36.0 to 0.45.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/93

## [v2.2.1](https://github.com/acidlemon/mirage-ecs/compare/v2.2.0...v2.2.1) - 2025-03-25

- fix example config. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/80
- Bump golang.org/x/crypto from 0.21.0 to 0.31.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/81
- Bump golang.org/x/net from 0.23.0 to 0.33.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/82
- Bump github.com/golang-jwt/jwt/v4 from 4.5.1 to 4.5.2 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/84
- bump up golang.org/x/net from 0.33.0 to 0.36.0 by @acidlemon in https://github.com/acidlemon/mirage-ecs/pull/85
- update builder Dockerfile go-version by @acidlemon in https://github.com/acidlemon/mirage-ecs/pull/86

## [v2.2.0](https://github.com/acidlemon/mirage-ecs/compare/v2.1.0...v2.2.0) - 2024-11-08

- add ecs:TagResource by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/75
- fix typo by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/76
- Bump github.com/golang-jwt/jwt/v4 from 4.4.3 to 4.5.1 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/78
- Refactoring. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/77
- Add purge configuration and scheduled purger functionality by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/79

## [v2.1.0](https://github.com/acidlemon/mirage-ecs/compare/v2.0.6...v2.1.0) - 2024-09-24

- add exclude_regexp to /api/purge by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/74

## [v2.0.6](https://github.com/acidlemon/mirage-ecs/compare/v2.0.5...v2.0.6) - 2024-08-26

- use t.Logf by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/71
- Fix panic when fields of Failures are nil by @kofuk in https://github.com/acidlemon/mirage-ecs/pull/72
- fix goreleaser by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/73

## [v2.0.5](https://github.com/acidlemon/mirage-ecs/compare/v2.0.4...v2.0.5) - 2024-05-10

- Bump golang.org/x/net from 0.17.0 to 0.23.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/68
- Fix: transport breaks large contents by setting a timeout. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/70

## [v2.0.4](https://github.com/acidlemon/mirage-ecs/compare/v2.0.3...v2.0.4) - 2024-04-09

- Don't reuse a tracer instance. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/67

## [v2.0.3](https://github.com/acidlemon/mirage-ecs/compare/v2.0.1...v2.0.3) - 2024-03-29

- Fix: panic when config.ecs.launch_type is nil by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/66

## [v2.0.2](https://github.com/acidlemon/mirage-ecs/compare/v1.1.1...v2.0.2) - 2024-01-25

- Bump golang.org/x/crypto from 0.14.0 to 0.17.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/52
- use HTMX! by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/51
- fix invalid warning by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/54
- Add tracer, show stopped task in webapi list. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/55
- -local mode don't access to ECS. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/49

## [v2.0.1](https://github.com/acidlemon/mirage-ecs/compare/v2.0.0...v2.0.1) - 2024-03-14

- set ldflags to embed Version and buildDate by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/65

## [v2.0.0](https://github.com/acidlemon/mirage-ecs/compare/v2.0.2...v2.0.0) - 2024-02-09

- Fix/csrf more by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/56
- API accespts application/json only. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/57
- switch to log/slog by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/58
- v2 doesn't base64 encode a subdomain value. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/59
- fix Origin header validation. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/60
- Fix/v2 tf by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/61
- fix readme and images by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/62
- Go 1.22 by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/63
- release build by go 1.22 by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/64

## [v1.1.1](https://github.com/acidlemon/mirage-ecs/compare/v1.1.0...v1.1.1) - 2023-11-01

- Pass OPTIONS requests without cookie auth. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/50

## [v1.1.0](https://github.com/acidlemon/mirage-ecs/compare/v1.0.2...v1.1.0) - 2023-10-24

- add requre_auth_cookie feature. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/47

## [v1.0.2](https://github.com/acidlemon/mirage-ecs/compare/v1.0.1...v1.0.2) - 2023-10-23

- Don't set SubdomainRaw tag. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/46

## [v1.0.1](https://github.com/acidlemon/mirage-ecs/compare/v1.0.0...v1.0.1) - 2023-10-17

- Set SubdomainRaw tag and environment variable. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/45
- Bump golang.org/x/net from 0.12.0 to 0.17.0 by @dependabot[bot] in https://github.com/acidlemon/mirage-ecs/pull/44

## [v1.0.0](https://github.com/acidlemon/mirage-ecs/compare/v0.99.3...v1.0.0) - 2023-09-27

- Load config and html files from S3. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/42
- Add auth configuration by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/43
- Transport timeout by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/40

## [v0.99.3](https://github.com/acidlemon/mirage-ecs/compare/v0.99.2...v0.99.3) - 2023-09-26

- html download from s3 by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/41

## [v0.99.2](https://github.com/acidlemon/mirage-ecs/compare/v0.99.1...v0.99.2) - 2023-08-21

- /api/purge durtaion allows integer or string. by @fujiwara in https://github.com/acidlemon/mirage-ecs/pull/39
