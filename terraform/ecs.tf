resource "aws_ecs_cluster" "mirage-ecs" {
  name = "mirage-ecs"
  tags = {
    Name = "mirage-ecs"
  }
}
