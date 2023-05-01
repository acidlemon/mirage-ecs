resource "aws_cloudwatch_log_group" "mirage-ecs" {
  name = "/aws/ecs/${var.project}"
}
