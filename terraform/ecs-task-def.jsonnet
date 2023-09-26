{
  cpu: '256',
  memory: '512',
  containerDefinitions: [
    {
      name: 'mirage-ecs',
      image: 'ghcr.io/acidlemon/mirage-ecs:v0.99.3',
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
          value: '{{ tfstate `aws_route53_zone.mirage-ecs.name` }}',
        },
        {
          name: 'MIRAGE_LOG_LEVEL',
          value: 'info',
        },
        {
          name: 'MIRAGE_CONF',
          value: 's3://{{ tfstate `aws_s3_bucket.mirage-ecs.bucket` }}/config.yaml'
        },
        {
          name: 'HTMLDIR',
          value: 's3://{{ tfstate `aws_s3_bucket.mirage-ecs.bucket` }}/html'
        },
      ],
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-group': '{{ tfstate `aws_cloudwatch_log_group.mirage-ecs.name` }}',
          'awslogs-region': '{{ must_env `AWS_REGION` }}',
          'awslogs-stream-prefix': 'mirage-ecs',
        },
      },
    },
  ],
  family: 'mirage-ecs',
  taskRoleArn: '{{ tfstate `aws_iam_role.task.arn` }}',
  executionRoleArn: '{{ tfstate `data.aws_iam_role.ecs-task-execiton.arn` }}',
  networkMode: 'awsvpc',
  requiresCompatibilities: [
    "EC2",
    "FARGATE",
  ],
}
