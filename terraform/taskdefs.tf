// Task role for tasks launched by mirage-ecs.
// mirage-ecs runs tasks with execute command enabled, which requires
// a task role that allows ssmmessages.
resource "aws_iam_role" "target-task" {
  name = "${var.project}-target-task"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Principal = {
          Service = "ecs-tasks.amazonaws.com"
        }
        Effect = "Allow"
        Sid    = ""
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "target-task-exec" {
  role       = aws_iam_role.target-task.name
  policy_arn = aws_iam_policy.mirage-ecs-exec.arn
}

// Task definitions launched by mirage-ecs for testing.
// e.g. curl https://mirage.<domain>/api/launch -d subdomain=test1 -d branch=main -d taskdef=nginx
resource "aws_ecs_task_definition" "target" {
  for_each                 = toset(["nginx", "httpd", "caddy"])
  family                   = each.key
  cpu                      = "256"
  memory                   = "512"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.target-task.arn

  container_definitions = jsonencode([
    {
      name      = each.key
      image     = "${each.key}:latest"
      essential = true
      portMappings = [
        {
          containerPort = 80
          hostPort      = 80
          protocol      = "tcp"
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.mirage-ecs.name
          "awslogs-region"        = var.region
          "awslogs-stream-prefix" = each.key
        }
      }
    }
  ])
}
