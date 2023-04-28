{
  deploymentConfiguration: {
    deploymentCircuitBreaker: {
      enable: false,
      rollback: false,
    },
    maximumPercent: 200,
    minimumHealthyPercent: 100,
  },
  deploymentController: {
    type: 'ECS',
  },
  desiredCount: 1,
  enableECSManagedTags: false,
  enableExecuteCommand: true,
  healthCheckGracePeriodSeconds: 0,
  launchType: 'FARGATE',
  loadBalancers: [
    {
      containerName: 'mirage-ecs',
      containerPort: 80,
      targetGroupArn: '{{ tfstate `aws_lb_target_group.mirage-ecs-http.arn` }}',
    },
  ],
  networkConfiguration: {
    awsvpcConfiguration: {
      assignPublicIp: 'ENABLED',
      securityGroups: [
        '{{ tfstate `aws_security_group.default.id` }}',
      ],
      subnets: [
        '{{ tfstate `aws_subnet.public-a.id` }}',
        '{{ tfstate `aws_subnet.public-c.id` }}',
        '{{ tfstate `aws_subnet.public-d.id` }}',
      ],
    },
  },
  platformFamily: 'Linux',
  platformVersion: 'LATEST',
  propagateTags: 'SERVICE',
  runningCount: 0,
  schedulingStrategy: 'REPLICA',
  tags: [
    {
      key: 'env',
      value: 'mirage-ecs',
    },
  ],
}
