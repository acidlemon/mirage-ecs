## An example of mirage-ecs deployment using terraform

This example shows how to deploy mirage-ecs using terraform.

### Prerequisites

- [Terraform](https://www.terraform.io/) >= v1.0.0
- [ecspresso](https://github.com/kayac/ecspresso) >= v2.0.0

#### Environment variables

- `AWS_REGION` for AWS region. (e.g. `ap-northeast-1`)
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, or `AWS_PROFILE` for AWS credentials.
- `AWS_SDK_LOAD_CONFIG=true` may be required if you use `AWS_PROFILE` and `~/.aws/config`.

### Usage

```console
$ terraform init
$ terraform apply -var domain=dev.your.example.com
$ ecspresso deploy
```

While applying terraform, `dev.your.example.com` will be registered to Route53.
You should delegate `dev.your.example.com` to the name servers from `your.example.com`.

After deploying, you can access to `https://mirage.dev.your.example.com` and see the mirage-ecs.

### Cleanup

```console
$ ecspresso delete --terminate
$ terraform destroy -var domain=dev.your.example.com
```
