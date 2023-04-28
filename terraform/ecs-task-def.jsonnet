{
  cpu: '256',
  memory: '512',
  containerDefinitions: [
    {
      name: 'mirage-ecs',
      image: 'ghcr.io/acidlemon/mirage-ecs:v0.7.1',
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
          value: 'debug',
        },
      ],
      logConfiguration: {
        logDriver: 'awslogs',
        options: {
          'awslogs-create-group': 'true',
          'awslogs-group': '/ecs/mirage-ecs',
          'awslogs-region': 'ap-northeast-1',
          'awslogs-stream-prefix': 'ecs',
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
