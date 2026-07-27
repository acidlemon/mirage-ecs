local env = std.native('env');
local must_env = std.native('must_env');
local tfstate = std.native('tfstate');
local bucket = tfstate('aws_s3_bucket.mirage-ecs.bucket');
// IMAGE overrides the whole image URI (e.g. a locally built image pushed to ECR).
local image = env('IMAGE', 'ghcr.io/acidlemon/mirage-ecs:' + env('VERSION', 'v2.2.4'));
{
  family: 'mirage-ecs',
  cpu: '256',
  memory: '512',
  networkMode: 'awsvpc',
  requiresCompatibilities: [
    'EC2',
    'FARGATE',
  ],
  taskRoleArn: tfstate('aws_iam_role.task.arn'),
  executionRoleArn: tfstate('aws_iam_role.execution.arn'),
  containerDefinitions: [
    {
      name: 'mirage-ecs',
      image: image,
      portMappings: [
        {
          containerPort: 80,
          hostPort: 80,
          protocol: 'tcp',
        },
      ],
      essential: true,
      environment: [
        {
          name: 'MIRAGE_DOMAIN',
          value: tfstate('data.aws_route53_zone.main.name'),
        },
        {
          name: 'MIRAGE_LOG_LEVEL',
          value: env('LOG_LEVEL', 'info'),
        },
        {
          name: 'MIRAGE_CONF',
          value: 's3://%s/config.yaml' % bucket,
        },
        {
          name: 'HTMLDIR',
          value: 's3://%s/html' % bucket,
        },
      ],
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': tfstate('aws_cloudwatch_log_group.mirage-ecs.name'),
          'awslogs-region': must_env('AWS_REGION'),
          'awslogs-stream-prefix': 'mirage-ecs',
        },
      },
    },
  ],
}
