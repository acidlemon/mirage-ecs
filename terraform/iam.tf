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
          "ecs:TagResource",
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
      },
      {
        Action = [
          "s3:GetObject",
        ],
        Effect = "Allow",
        Resource = [
          "${aws_s3_bucket.mirage-ecs.arn}/*",
        ]
      },
      {
        Action = [
          "s3:ListBucket",
        ],
        Effect = "Allow",
        Resource = [
          "${aws_s3_bucket.mirage-ecs.arn}",
        ]
      },
    ]
  })
}

resource "aws_iam_role_policy_attachment" "mirage-ecs" {
  role       = aws_iam_role.task.name
  policy_arn = aws_iam_policy.mirage-ecs.arn
}

resource "aws_iam_role" "execution" {
  name = "${var.project}-ecs-execution"
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

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

// Allows ECS Exec into containers. Attached to the mirage-ecs task role
// and the task role of tasks launched by mirage-ecs.
resource "aws_iam_policy" "mirage-ecs-exec" {
  name = "${var.project}-ecs-exec"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssmmessages:CreateControlChannel",
          "ssmmessages:CreateDataChannel",
          "ssmmessages:OpenControlChannel",
          "ssmmessages:OpenDataChannel",
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "mirage-ecs-exec" {
  role       = aws_iam_role.task.name
  policy_arn = aws_iam_policy.mirage-ecs-exec.arn
}
