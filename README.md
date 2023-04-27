mirage-ecs - reverse proxy frontend for Amazon ECS
===========================================

mirage-ecs is reverse proxy for ECS task and task manager.

mirage-ecs can run and stop ECS task and serve http request with specified subdomain. Additionaly, mirage passes variable to containers in task using environment variables.

Usage
------

1. Setup mirage-ecs and edit configuration (see Setup section for detail.)
2. Run mirage-ecs on ECS cluster.

Following instructions use below settings.

```
host:
  webapi: docker.dev.example.net
  reverse_proxy_suffix: .dev.example.net
listen:
  HTTP: 80
```

Prerequisite: you should resolve `*.dev.example.net` to your ECS task.

### Requirements

mirage-ecs requires [ECS Long ARN Format](https://aws.amazon.com/jp/blogs/compute/migrating-your-amazon-ecs-deployment-to-the-new-arn-and-resource-id-format-2/) for tagging tasks.

If your account do not enable these settings yet, you must enable that.

```console
$ aws ecs put-account-setting-default --name taskLongArnFormat --value enabled
```

### Using CLI

3. Launch ECS task container using curl.
```
curl http://docker.dev.example.net/api/launch \
  -d subdomain=cool-feature \
  -d branch=feature/cool \
  -d taskdef=myapp
```
4. Now, you can access to container using "http://cool-feature.dev.exmaple.net/".

5. Terminate the task using curl.
```
curl http://docker.dev.example.net/api/terminate \
  -d subdomain=cool-feature
```

`subdomain` supports wildcard (e.g. `www*`,`foo[0-9]`, `api-?-test`).
Mirage matches the pattern to hostname using Go's [path/#Match](https://golang.org/pkg/path/#Match).

### Using Web Interface

3. Access to mirage web interface via "http://docker.dev.example.net/".
4. Press "Launch New Task".
5. Fill launch options.
  - subdomain: cool-feature
  - branch: feature/cool
  - taskdef: myapp
6. Now, you can access to container using "http://cool-feature.dev.exmaple.net/".
7. Press "Terminate" button.

### Customization

mirage-ecs now supports custom parameter. Write your parameter on config.yml.

mirage-ecs contains default parameter "branch" as below.

```
parameters:
  - name: branch
    env: GIT_BRANCH
    rule: ""
    required: true
```

You can add custom parameter. "rule" option is regexp string.

### API Documents

#### `GET /api/list`

`/api/list` returns list of running tasks.

```json
{
    "result": [
        {
            "id": "arn:aws:ecs:ap-northeast-1:12345789012:task/dev/af8e7a6dad6e44d4862696002f41c2dc",
            "short_id": "af8e7a6dad6e44d4862696002f41c2dc",
            "subdomain": "b15",
            "branch": "topic/b15",
            "taskdef": "dev:756",
            "ipaddress": "10.206.242.48",
            "created": "0001-01-01T00:00:00Z",
            "last_status": "PENDING",
            "port_map": {
                "nginx": 80
            },
            "env": {
                "GIT_BRANCH": "topic/b15",
                "SUBDOMAIN": "YjE1",
            }
        },
        {
            "id": "arn:aws:ecs:ap-northeast-1:123456789012:task/dev/d007a00bf9a0411ebbcf95291aced40f",
            "short_id": "d007a00bf9a0411ebbcf95291aced40f",
            "subdomain": "bench",
            "branch": "feature/bench",
            "taskdef": "dev:641",
            "ipaddress": "10.206.240.60",
            "created": "2023-03-13T00:29:08.959Z",
            "last_status": "RUNNING",
            "port_map": {
                "nginx": 80
            },
            "env": {
                "GIT_BRANCH": "feature/bench",
                "SUBDOMAIN": "YmVuY2g=",
            }
        }
    ]
}
```

#### `POST /api/launch`

`/api/launch` launches a new task.

Parameters:
- `subdomain`: subdomain of the task.
- `branch`: branch name for the task.
- `taskdef`: ECS task definition name (maybe includes revision) for the task.

```json
{
  "status": "ok"
}
```

#### `GET /api/logs`

`/api/logs` returns logs of the task.

Parameters:
- `subdomain`: subdomain of the task.
- `since`: RFC3339 timestamp of the first log to return.
- `tail`: number of lines to return or `all`.

```json
{
    "result": [
      "2023/03/13 00:29:08 [notice] 1#1: using the \"epoll\" event method",
      "2023/03/13 00:29:08 [notice] 1#1: nginx/1.11.10",
    ]
}
```

#### `POST /api/terminate`

`/api/terminate` terminates the task.

Parameters:
- `subdomain`: subdomain of the task.
- `id`: task ID of the task.

`subdomain` and `id` are exclusive. If both are specified, `id` is used.

```json
{
  "status": "ok"
}
```

#### `GET /api/access`

`/api/access` returns access counter of the task.

Parameters:
- `subdomain`: subdomain of the task.
- `duration`: duration(seconds) of the counter. default is 86400.

```json
{
  "result": "ok",
  "duration": 86400,
  "sum": 123
}
```

### mirage link

mirage link feature enables to launch and terminate multiple tasks that have the same subdomain.

mirage link works as below.
- Launch API launches multiple tasks that have the same subdomain.
- mirage-ecs puts to DNS name of these tasks into Route53 hosted zone.
  -  e.g. `{container-name}.{subdomain}.{hosted-zone} A {tasks IP address}`

For example,
- hosted zone: `mirage.example.com``
- First task (IP address 10.1.0.1) has container `proxy`.
- Second task (IP address 10.2.0.2) has container `app`.
- Subdomain: `myenv`

mirage-ecs puts the following DNS records.
- `nginx.myenv.mirage.example.com A 10.1.0.1`
- `app.myenv.mirage.example.com A 10.2.0.2`

So the proxy container can connect to the app with the DNS name `app.myenv.mirage.example.com`.

To enable mirage link, define your Route53 hosted zone ID in a config.

```yaml
link:
  hosted_zone_id: your route53 hosted zone ID
```

Setup
------

In docker/ directory,

1. Edit `config.yml` to your environment.
1. Do `make` to create a docker image.
1. Push the image to ECR.
1. Put mirage-ecs task definition to ECS.
   - See also [mirage-ecs-taskdef.json](mirage-ecs-taskdef.json)
1. Run mirage-ecs service in your ECS.

License
--------

The MIT License (MIT)

(c) 2019 acidlemon. (c) 2019 KAYAC Inc. (c) 2019 fujiwara.
