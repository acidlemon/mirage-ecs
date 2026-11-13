#!/bin/bash
# Creates the AWS resources for the e2e test on an AWS emulator, and writes
# a mirage-ecs config file which refers to them.
#
#   setup.sh <config file to write>
#
# Do not run this directly. test/e2e/run.sh sets the environment variables
# which point the AWS CLI at the emulator.
set -euo pipefail

config="$1"
dir="$(cd "$(dirname "$0")" && pwd)"

if [[ -z "${AWS_ENDPOINT_URL:-}" ]]; then
  echo "AWS_ENDPOINT_URL is not set. Refusing to run against real AWS." >&2
  exit 1
fi

name=mirage-e2e
domain=e2e.example.net

text() {
  aws "$@" --output text
}

# The roles are required by the awslogs log driver and ECS Exec.
role_arn=$(text iam get-role --role-name "$name" --query Role.Arn 2>/dev/null ||
  text iam create-role --role-name "$name" --query Role.Arn \
    --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs-tasks.amazonaws.com"},"Action":"sts:AssumeRole"}]}')

vpc_id=$(text ec2 describe-vpcs --filters "Name=tag:Name,Values=$name" --query 'Vpcs[0].VpcId')
if [[ "$vpc_id" == "None" ]]; then
  vpc_id=$(text ec2 create-vpc --cidr-block 10.0.0.0/16 --query Vpc.VpcId \
    --tag-specifications "ResourceType=vpc,Tags=[{Key=Name,Value=$name}]")
  text ec2 create-subnet --vpc-id "$vpc_id" --cidr-block 10.0.1.0/24 --query Subnet.SubnetId
  text ec2 create-security-group --vpc-id "$vpc_id" --group-name "$name" --description "$name" --query GroupId
  # A task fails to pull its image unless the subnet has a route to the internet.
  igw_id=$(text ec2 create-internet-gateway --query InternetGateway.InternetGatewayId)
  aws ec2 attach-internet-gateway --internet-gateway-id "$igw_id" --vpc-id "$vpc_id"
  rtb_id=$(text ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpc_id" --query 'RouteTables[0].RouteTableId')
  text ec2 create-route --route-table-id "$rtb_id" --destination-cidr-block 0.0.0.0/0 --gateway-id "$igw_id"
fi
subnet_id=$(text ec2 describe-subnets --filters "Name=vpc-id,Values=$vpc_id" --query 'Subnets[0].SubnetId')
sg_id=$(text ec2 describe-security-groups --filters "Name=vpc-id,Values=$vpc_id" "Name=group-name,Values=$name" --query 'SecurityGroups[0].GroupId')

text ecs create-cluster --cluster-name "$name" --query cluster.clusterArn

# The log group must exist, otherwise the logs of tasks are not delivered.
if [[ "$(text logs describe-log-groups --log-group-name-prefix "/ecs/$name" --query 'length(logGroups)')" == "0" ]]; then
  aws logs create-log-group --log-group-name "/ecs/$name"
fi

text ecs register-task-definition --family "$name" \
  --network-mode awsvpc --requires-compatibilities FARGATE --cpu 256 --memory 512 \
  --execution-role-arn "$role_arn" --task-role-arn "$role_arn" \
  --container-definitions "file://$dir/container-definitions.json" \
  --query taskDefinition.taskDefinitionArn

zone_id=$(text route53 list-hosted-zones-by-name --dns-name "$domain" \
  --query "HostedZones[?Name=='$domain.'] | [0].Id")
if [[ "$zone_id" == "None" ]]; then
  zone_id=$(text route53 create-hosted-zone --name "$domain" --caller-reference "$name-$(date +%s)" --query HostedZone.Id)
fi
zone_id="${zone_id##*/}"

cat > "$config" <<YAML
host:
  webapi: mirage.$domain
  reverse_proxy_suffix: .$domain
listen:
  foreign_address: 127.0.0.1
  http:
    - listen: ${MIRAGE_E2E_PORT:-18080}
      target: 80
parameters:
  - name: branch
    env: GIT_BRANCH
    required: true
ecs:
  region: $AWS_REGION
  cluster: $name
  launch_type: FARGATE
  enable_execute_command: true
  network_configuration:
    awsvpc_configuration:
      subnets:
        - $subnet_id
      security_groups:
        - $sg_id
      assign_public_ip: ENABLED
link:
  hosted_zone_id: $zone_id
YAML
echo "$zone_id"
