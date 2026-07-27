## An example of mirage-ecs deployment using terraform

This example shows how to deploy mirage-ecs using terraform and ecspresso.

This is designed as a disposable environment for trying out mirage-ecs: the main state can be created and destroyed repeatedly, while the Route53 hosted zone lives in a separate long-lived state (`dns/`) so that you do not have to re-delegate the domain every time.

### Prerequisites

- [Terraform](https://www.terraform.io/) >= v1.8.0
- [ecspresso](https://github.com/kayac/ecspresso) >= v2.4.0 (the definition files use Jsonnet native functions)
- [tfstate-lookup](https://github.com/fujiwara/tfstate-lookup) (used by `Makefile`)

#### Environment variables

- `AWS_REGION` for AWS region. (e.g. `ap-northeast-1`)
- `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`, or `AWS_PROFILE` for AWS credentials.
- `AWS_SDK_LOAD_CONFIG=true` may be required if you use `AWS_PROFILE` and `~/.aws/config`.

### Setup (once): create the hosted zone

```console
$ cd dns
$ terraform init
$ terraform apply -var domain=dev.your.example.com
```

Delegate `dev.your.example.com` to the name servers shown in the `name_servers` output (add NS records to the parent zone `your.example.com`). This is needed only once; keep this state as long as you use this domain.

### Spin up

```console
$ terraform init
$ terraform apply -var domain=dev.your.example.com
$ ecspresso deploy
```

`terraform apply` creates a VPC, an ALB with an ACM certificate, an ECS cluster, IAM roles, an S3 bucket for the mirage-ecs configuration, and task definitions (`nginx`, `httpd`, `caddy`) for testing launches. `ecspresso deploy` deploys the mirage-ecs service itself.

The mirage-ecs image version can be specified by the `VERSION` environment variable (e.g. `VERSION=v2.2.4 ecspresso deploy`).

After deploying, you can access `https://mirage.dev.your.example.com` and see the mirage-ecs Web UI. Launch a task with taskdef `nginx` (or `httpd`, `caddy`) and subdomain `test1`, then access `https://test1.dev.your.example.com`.

Using the CLI:

```console
$ curl https://mirage.dev.your.example.com/api/launch \
  -d subdomain=test1 -d branch=main -d taskdef=nginx
$ curl https://test1.dev.your.example.com/
$ curl https://mirage.dev.your.example.com/api/terminate -d subdomain=test1
```

#### Testing a locally built image

To verify changes in your working tree (e.g. before merging a pull request), build the image locally, push it to the ECR repository created by terraform, and deploy it:

```console
$ make deploy/image
```

This builds the image from the repository root with `docker/Dockerfile`, tags it with the current git commit hash (override with `TAG=...`), pushes it to ECR, and deploys with the `IMAGE` environment variable pointing to it.

To go back to the official image, run `make deploy` (or `ecspresso deploy`) without `IMAGE`.

#### Customization

- `terraform apply -var project=... -var region=...` changes the resource name prefix and the region.
- `IMAGE` overrides the whole image URI of mirage-ecs. Without it, the official image `ghcr.io/acidlemon/mirage-ecs` with the `VERSION` tag is used.
- `config.yaml` is the mirage-ecs configuration uploaded to S3. After editing it, run `make deploy/config` (or `terraform apply`) to upload. `make diff` / `make deploy` wrap `ecspresso diff` / `ecspresso deploy` as well.
- This example does not enable any authentication. To restrict access, use mirage-ecs token authentication (`auth` section in `config.yaml` with `MIRAGE_TOKEN`) or ALB listener rules. See the top-level README for details.

### Tear down

```console
$ ecspresso delete --terminate
$ terraform destroy -var domain=dev.your.example.com
```

The `dns/` state is kept, so you can spin up the environment again without re-delegation.
