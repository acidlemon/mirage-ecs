// ECR repository for testing locally built mirage-ecs images.
// See "Testing a locally built image" in README.md.
resource "aws_ecr_repository" "mirage-ecs" {
  name         = var.project
  force_delete = true
}
