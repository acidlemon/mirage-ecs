resource "aws_ecs_cluster" "mirage-ecs" {
  name = var.project
  tags = {
    Name = var.project
  }
}
