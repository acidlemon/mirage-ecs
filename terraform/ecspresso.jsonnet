local must_env = std.native('must_env');
{
  region: must_env('AWS_REGION'),
  // must be the same as var.project in the terraform configuration
  cluster: 'mirage-ecs',
  service: 'mirage-ecs',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  timeout: '10m0s',
  plugins: [
    {
      name: 'tfstate',
      config: {
        path: 'terraform.tfstate',
      },
    },
  ],
}
