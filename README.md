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
  -d taskdef=arn:aws:ecs:ap-northeast-1:123456789012:task-definition/myapp
```
4. Now, you can access to container using "http://cool-feature.dev.exmaple.net/".

5. Terminate the task using curl.
```
curl http://docker.dev.example.net/api/terminate \
  -d subdomain=cool-feature
```

### Using Web Interface

3. Access to mirage web interface via "http://docker.dev.example.net/".
4. Press "Launch New Task".
5. Fill launch options.
  - subdomain: cool-feature
  - branch: feature/cool
  - taskdef: arn:aws:ecs:ap-northeast-1:123456789012:task-definition/myapp
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
