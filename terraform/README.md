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

#### Customization

You can customize the deployment by editing `terraform.tfvars` and `ecspresso.yml`.

`oauth_client_id` and `oauth_client_secret` are used for authentication by ALB with Google OAuth.
If you want to enable authentication, you should set them.
Set the Google OAuth callback URL to `https://mirage.{var.domain}/oauth2/idresponse`.

`ecspresso.yml` is used for ECS deployment.
See [ecspresso](https://github.com/kayac/ecspresso) for details.

### Cleanup

```console
$ ecspresso delete --terminate
$ terraform destroy -var domain=dev.your.example.com
```
