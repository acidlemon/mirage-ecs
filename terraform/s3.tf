// bucket_prefix generates a unique bucket name for each apply, to avoid
// the delay in reusing a bucket name after deletion.
resource "aws_s3_bucket" "mirage-ecs" {
  bucket_prefix = "${var.project}-"
  force_destroy = true
}

resource "aws_s3_object" "config" {
  bucket = aws_s3_bucket.mirage-ecs.id
  key    = "config.yaml"
  source = "config.yaml"
}

resource "aws_s3_object" "html" {
  for_each = toset(["launcher.html", "layout.html", "list.html"])
  bucket   = aws_s3_bucket.mirage-ecs.id
  key      = "html/${each.value}"
  source   = format("../html/%s", each.value)
}
