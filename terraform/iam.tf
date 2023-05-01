resource "aws_iam_role" "task" {
  name = "${var.project}-ecs-task"
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

resource "aws_iam_policy" "mirage-ecs" {
  name = var.project
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "iam:PassRole",
          "ecs:RunTask",
          "ecs:DescribeTasks",
          "ecs:DescribeTaskDefinition",
          "ecs:DescribeServices",
          "ecs:StopTask",
          "ecs:ListTasks",
          "cloudwatch:PutMetricData",
          "cloudwatch:GetMetricData",
          "logs:GetLogEvents",
          "route53:GetHostedZone",
          "route53:ChangeResourceRecordSets",
        ]
        Effect   = "Allow"
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "mirage-ecs" {
  role       = aws_iam_role.task.name
  policy_arn = aws_iam_policy.mirage-ecs.arn
}

data "aws_iam_role" "ecs-task-execiton" {
  name = "ecsTaskExecutionRole"
}
