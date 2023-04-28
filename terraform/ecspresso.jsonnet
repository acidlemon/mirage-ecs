{
  region: 'ap-northeast-1',
  cluster: 'mirage-ecs',
  service: 'mirage-ecs',
  service_definition: 'ecs-service-def.jsonnet',
  task_definition: 'ecs-task-def.jsonnet',
  timeout: '10m0s',
  plugins: [
    {
      name: "tfstate",
      config: {
        url: "terraform.tfstate"
      },
    },
  ]
}
