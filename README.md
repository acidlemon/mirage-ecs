# mirage-ecs - reverse proxy frontend for Amazon ECS

mirage-ecs is reverse proxy for ECS task and task manager.

mirage-ecs can run and stop an ECS task and serve http request with specified subdomain. Additionaly, mirage passes variable to containers in task using environment variables.

## Usage

Build mirage-ecs container with a configuration file. (default `config.yaml`)

Run mirage-ecs as ECS service.

### Minimal Configuration

```yaml
host:
  webapi: mirage.dev.example.net         # hostname of mirage-ecs webapi
  reverse_proxy_suffix: .dev.example.net # suffix of launched ECS task hostname
```

1. mirage-ecs accepts HTTP request on "http://mirage.dev.example.net".
2. Launch ECS task container specified subdomain.
3. Now, you can access to the task using "http://<subdomain>.dev.exmaple.net/".

`*.dev.example.net` should be resolved to mirage-ecs webapi.

### Full Configuration

#### `host` section

`host` section configures hostname of mirage-ecs webapi and suffix of launched ECS task hostname.

```yaml
host:
  webapi: mirage.dev.example.net         # hostname of mirage-ecs webapi
  reverse_proxy_suffix: .dev.example.net # suffix of launched ECS task hostname
```

#### `listen` section

`listen` section configures port number of mirage-ecs webapi and target ECS task.

```yaml
listen:
  http:
    - listen: 80 # port number of mirage-ecs webapi
      target: 80 # port number of target ECS task
```

#### `parameters` section

`parameters` section configures parameters for ECS task.

The default parameter "branch" as below.

```yaml
parameters:
  - name: branch
    env: GIT_BRANCH
    rule: ""
    required: true
```

You can add any custom parameters. "rule" option is regexp string.

These parameters are passed to ECS task as environment variables.

#### `ecs` section

mirage-ecs configures `ecs:` section automatically based on the ECS service and task of itself.

If you want to partially override the settings, write the `ecs:` section.

```yaml
ecs:
  region: "ap-northeast-1"
  cluster: mycluster
  launch_type: FARGATE
  network_configuration:
    awsvpc_configuration:
      subnets:
        - subnet-aaaa0000
        - subnet-bbbb1111
        - subnet-cccc2222
      security_groups:
        - sg-11112222
        - sg-aaaagggg
      assign_public_ip: ENABLED
```

### Using CLI

Launch ECS task container using curl.

```console
$ curl http://mirage.dev.example.net/api/launch \
  -d subdomain=cool-feature \
  -d branch=feature/cool \
  -d taskdef=myapp
```

Terminate the task using curl.

```console
$ curl http://mirage.dev.example.net/api/terminate \
  -d subdomain=cool-feature
```

`subdomain` supports wildcard (e.g. `www*`,`foo[0-9]`, `api-?-test`).

mirage-ecs matches the pattern to hostname using Go's [path/#Match](https://golang.org/pkg/path/#Match).

### Using Web Interface

1. Access to mirage web interface via "http://mirage.dev.example.net/".
1. Press "Launch New Task".
1. Fill launch options.
  - subdomain: cool-feature
  - branch: feature/cool
  - taskdef: myapp
1. Now, you can access to container using "http://cool-feature.dev.exmaple.net/".
1. Press "Terminate" button.

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


### Requirements

mirage-ecs requires [ECS Long ARN Format](https://aws.amazon.com/jp/blogs/compute/migrating-your-amazon-ecs-deployment-to-the-new-arn-and-resource-id-format-2/) for tagging tasks.

If your account do not enable these settings yet, you must enable that.

```console
$ aws ecs put-account-setting-default --name taskLongArnFormat --value enabled
```

License
--------

The MIT License (MIT)

(c) 2019 acidlemon. (c) 2019 KAYAC Inc. (c) 2019 fujiwara.
